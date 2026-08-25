use crate::api::client::{CreateSecretRequest, UpdateSecretRequest};
use crate::api::ApiClient;
use anyhow::{bail, Context, Result};

/// What `set` may do with the key.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum SetMode {
    /// Create the secret, or update its value if the key exists (default)
    Upsert,
    /// Fail if the key already exists
    CreateOnly,
    /// Fail if the key does not exist
    UpdateOnly,
}

impl SetMode {
    pub fn from_flags(create_only: bool, update_only: bool) -> Self {
        match (create_only, update_only) {
            (true, _) => SetMode::CreateOnly,
            (_, true) => SetMode::UpdateOnly,
            _ => SetMode::Upsert,
        }
    }
}

/// Splits `KEY=value` at the first `=`; the value may contain further `=` signs.
pub fn parse_assignment(secret: &str) -> Result<(String, String)> {
    let (key, value) = secret
        .split_once('=')
        .context("Invalid format. Expected KEY=value")?;
    let key = key.trim();
    if key.is_empty() {
        bail!("Key name cannot be empty");
    }
    Ok((key.to_string(), value.to_string()))
}

#[allow(clippy::too_many_arguments)]
pub async fn execute(
    client: &ApiClient,
    secret: &str,
    vault: &str,
    env_name: &str,
    description: Option<&str>,
    rotation_days: Option<i32>,
    mode: SetMode,
) -> Result<()> {
    let (key_name, value) = parse_assignment(secret)?;

    // Resolve vault name to ID
    let vault_id = if let Some(v) = client.find_vault_by_name(vault).await? {
        v.id
    } else {
        vault.to_string()
    };

    // Resolve environment name to ID
    let env_obj = client
        .find_environment_by_name(&vault_id, env_name)
        .await?
        .context(format!(
            "Environment '{}' not found in vault '{}'",
            env_name, vault
        ))?;

    let existing = client
        .list_secrets(&env_obj.id)
        .await?
        .into_iter()
        .find(|s| s.key_name == key_name);

    match (existing, mode) {
        (Some(_), SetMode::CreateOnly) => {
            bail!("Secret '{key_name}' already exists in {vault}/{env_name} (--create-only)")
        }
        (None, SetMode::UpdateOnly) => {
            bail!("Secret '{key_name}' does not exist in {vault}/{env_name} (--update-only)")
        }
        (Some(current), _) => {
            // Keep everything not given on the command line
            let request = UpdateSecretRequest {
                value: Some(value),
                description: description.map(String::from).or(current.description),
                rotation_interval_days: rotation_days.or(current.rotation_interval_days),
                expires_at: current.expires_at,
                metadata: current.metadata,
            };
            client.update_secret(&current.id, &request).await?;
            println!("Secret '{key_name}' updated in {vault}/{env_name}.");
        }
        (None, _) => {
            let request = CreateSecretRequest {
                key_name: key_name.clone(),
                value,
                description: description.map(String::from),
                rotation_interval_days: rotation_days,
                expires_at: None,
                metadata: None,
            };
            let created = client.create_secret(&env_obj.id, request).await?;
            println!("Secret '{key_name}' created in {vault}/{env_name}.");
            println!("ID: {}", created.id);
        }
    }

    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn splits_at_the_first_equals_sign() {
        assert_eq!(
            parse_assignment("DATABASE_URL=postgres://u:p@h/db?sslmode=require").unwrap(),
            (
                "DATABASE_URL".into(),
                "postgres://u:p@h/db?sslmode=require".into()
            )
        );
        assert_eq!(
            parse_assignment("EMPTY=").unwrap(),
            ("EMPTY".into(), "".into())
        );
        assert!(parse_assignment("NO_EQUALS").is_err());
        assert!(parse_assignment("=value").is_err());
    }

    #[test]
    fn mode_flags() {
        assert_eq!(SetMode::from_flags(false, false), SetMode::Upsert);
        assert_eq!(SetMode::from_flags(true, false), SetMode::CreateOnly);
        assert_eq!(SetMode::from_flags(false, true), SetMode::UpdateOnly);
    }

    use crate::config::Tokens;
    use wiremock::matchers::{body_json, method, path};
    use wiremock::{Mock, MockServer, ResponseTemplate};

    async fn server_with_existing_secret() -> MockServer {
        let server = MockServer::start().await;
        Mock::given(method("GET"))
            .and(path("/vaults"))
            .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!([
                {"id": "v1", "name": "payments", "description": null, "created_at": "2026-01-01T00:00:00Z"}
            ])))
            .mount(&server)
            .await;
        Mock::given(method("GET"))
            .and(path("/vaults/v1/envs"))
            .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!([
                {"id": "e1", "name": "production", "vault_id": "v1", "description": null}
            ])))
            .mount(&server)
            .await;
        Mock::given(method("GET"))
            .and(path("/envs/e1/secrets"))
            .respond_with(
                ResponseTemplate::new(200).set_body_json(serde_json::json!([{
                    "id": "s1", "key_name": "API_KEY", "description": "Stripe key",
                    "rotation_interval_days": 90, "expires_at": "2027-01-01T00:00:00Z",
                    "metadata": {"team_name": "payments"}
                }])),
            )
            .mount(&server)
            .await;
        server
    }

    fn client_for(server: &MockServer) -> ApiClient {
        ApiClient::with_tokens(
            server.uri(),
            Tokens {
                access_token: "a".into(),
                refresh_token: "r".into(),
                api_url: Some(server.uri()),
            },
        )
    }

    #[tokio::test]
    async fn updating_an_existing_key_keeps_its_metadata() {
        let server = server_with_existing_secret().await;
        Mock::given(method("PUT"))
            .and(path("/secrets/s1"))
            .and(body_json(serde_json::json!({
                "value": "sk_new",
                "description": "Stripe key",
                "rotation_interval_days": 30,
                "expires_at": "2027-01-01T00:00:00Z",
                "metadata": {"team_name": "payments"}
            })))
            .respond_with(
                ResponseTemplate::new(200)
                    .set_body_json(serde_json::json!({"id": "s1", "key_name": "API_KEY"})),
            )
            .expect(1)
            .mount(&server)
            .await;

        let client = client_for(&server);
        execute(
            &client,
            "API_KEY=sk_new",
            "payments",
            "production",
            None,
            Some(30),
            SetMode::Upsert,
        )
        .await
        .unwrap();
    }

    #[tokio::test]
    async fn create_only_refuses_existing_keys() {
        let server = server_with_existing_secret().await;
        let client = client_for(&server);
        let err = execute(
            &client,
            "API_KEY=x",
            "payments",
            "production",
            None,
            None,
            SetMode::CreateOnly,
        )
        .await
        .unwrap_err();
        assert!(err.to_string().contains("already exists"));

        let err = execute(
            &client,
            "NEW_KEY=x",
            "payments",
            "production",
            None,
            None,
            SetMode::UpdateOnly,
        )
        .await
        .unwrap_err();
        assert!(err.to_string().contains("does not exist"));
    }
}
