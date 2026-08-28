use crate::api::ApiClient;
use crate::cli::resolve;
use anyhow::{bail, Context, Result};

/// Environment variable holding a service token for non-interactive use (CI, servers).
pub const TOKEN_ENV: &str = "SECRETS_TOKEN";

/// All decrypted values of one environment.
pub struct EnvironmentSecrets {
    pub vault: String,
    pub environment: String,
    pub pairs: Vec<(String, String)>,
}

/// Loads every secret of an environment, either with a service token (which is scoped to one
/// environment, so --vault and --env are optional and only checked) or with the logged-in
/// session (both required; each value is revealed and audited).
pub async fn load(
    client: &ApiClient,
    token: Option<&str>,
    vault: Option<&str>,
    env: Option<&str>,
) -> Result<EnvironmentSecrets> {
    if let Some(token) = token.filter(|t| !t.trim().is_empty()) {
        let data = client
            .token_secrets(token.trim())
            .await
            .context("Service token authentication failed")?;
        check_scope("vault", vault, &data.vault_name)?;
        check_scope("environment", env, &data.environment_name)?;
        return Ok(EnvironmentSecrets {
            vault: data.vault_name,
            environment: data.environment_name,
            pairs: data.secrets.into_iter().map(|s| (s.key, s.value)).collect(),
        });
    }

    let (Some(vault), Some(env)) = (vault, env) else {
        bail!("--vault and --env are required unless {TOKEN_ENV} is set");
    };
    let (vault, env) = resolve::environment(client, vault, env).await?;
    let secrets = client.list_secrets(&env.id).await?;
    let mut pairs = Vec::with_capacity(secrets.len());
    for secret in &secrets {
        let value = client
            .reveal_secret(&secret.id)
            .await
            .with_context(|| format!("Failed to reveal secret '{}'", secret.key_name))?;
        pairs.push((secret.key_name.clone(), value));
    }
    Ok(EnvironmentSecrets {
        vault: vault.name,
        environment: env.name,
        pairs,
    })
}

/// A token can only read its own environment; a mismatching --vault/--env is a mistake worth stopping on.
fn check_scope(what: &str, requested: Option<&str>, actual: &str) -> Result<()> {
    match requested {
        Some(r) if r != actual => {
            bail!(
                "{TOKEN_ENV} is for {what} '{actual}', but --{} '{r}' was given",
                if what == "vault" { "vault" } else { "env" }
            )
        }
        _ => Ok(()),
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use wiremock::matchers::{header, method, path};
    use wiremock::{Mock, MockServer, ResponseTemplate};

    #[tokio::test]
    async fn service_tokens_read_their_environment_without_a_session() {
        let server = MockServer::start().await;
        Mock::given(method("GET"))
            .and(path("/token/secrets"))
            .and(header("authorization", "Bearer dsm_st_abc"))
            .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!({
                "vault_name": "app", "environment_name": "production", "token_name": "ci",
                "secrets": [{"key": "API_KEY", "value": "sk_live"}]
            })))
            .expect(2)
            .mount(&server)
            .await;
        // A client with no stored session at all
        let client = ApiClient::new(server.uri());

        let loaded = load(&client, Some("dsm_st_abc"), None, None).await.unwrap();
        assert_eq!(
            (loaded.vault.as_str(), loaded.environment.as_str()),
            ("app", "production")
        );
        assert_eq!(
            loaded.pairs,
            vec![("API_KEY".to_string(), "sk_live".to_string())]
        );

        let err = load(&client, Some("dsm_st_abc"), Some("app"), Some("staging"))
            .await
            .err()
            .unwrap();
        assert!(
            err.to_string().contains("is for environment 'production'"),
            "{err}"
        );
    }

    #[tokio::test]
    async fn session_mode_requires_vault_and_env() {
        let client = ApiClient::new("http://127.0.0.1:9");
        let err = load(&client, None, Some("app"), None).await.err().unwrap();
        assert!(err.to_string().contains("--vault and --env are required"));
    }
}
