use crate::api::ApiClient;
use crate::cli::resolve;
use crate::utils::{dotenv, write_private};
use anyhow::{Context, Result};
use std::path::Path;

pub async fn execute(client: &ApiClient, vault: &str, env: &str, out: Option<&str>) -> Result<()> {
    let (vault_obj, env_obj) = resolve::environment(client, vault, env).await?;

    // Get all secrets
    let secrets = client.list_secrets(&env_obj.id).await?;

    if secrets.is_empty() {
        println!(
            "No secrets found in environment '{}/{}'.",
            vault_obj.name, env_obj.name
        );
        return Ok(());
    }

    // Reveal all secret values
    let mut pairs = Vec::with_capacity(secrets.len());
    for secret in &secrets {
        let value = client
            .reveal_secret(&secret.id)
            .await
            .context(format!("Failed to reveal secret '{}'", secret.key_name))?;
        pairs.push((secret.key_name.clone(), value));
    }
    let output = dotenv::format(&pairs);

    // Write to file (readable only by you) or stdout
    if let Some(file_path) = out {
        write_private(Path::new(file_path), &output)?;
        println!("Secrets written to {} (mode 600)", file_path);
    } else {
        print!("{}", output);
    }

    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::config::Tokens;
    use wiremock::matchers::{method, path};
    use wiremock::{Mock, MockServer, ResponseTemplate};

    #[tokio::test]
    async fn pull_writes_quoted_values_to_a_private_file() {
        let server = MockServer::start().await;
        let json = |body: serde_json::Value| ResponseTemplate::new(200).set_body_json(body);
        Mock::given(method("GET")).and(path("/vaults"))
            .respond_with(json(serde_json::json!([{"id": "v1", "name": "app", "description": null, "created_at": "2026-01-01T00:00:00Z"}])))
            .mount(&server).await;
        Mock::given(method("GET")).and(path("/vaults/v1/envs"))
            .respond_with(json(serde_json::json!([{"id": "e1", "name": "staging", "vault_id": "v1", "description": null}])))
            .mount(&server).await;
        Mock::given(method("GET")).and(path("/envs/e1/secrets"))
            .respond_with(json(serde_json::json!([{"id": "s1", "key_name": "PLAIN"}, {"id": "s2", "key_name": "SPACED"}])))
            .mount(&server).await;
        Mock::given(method("POST")).and(path("/secrets/s1/reveal"))
            .respond_with(json(serde_json::json!({"id": "s1", "key_name": "PLAIN", "value": "abc123", "expires_in": 30})))
            .mount(&server).await;
        Mock::given(method("POST")).and(path("/secrets/s2/reveal"))
            .respond_with(json(serde_json::json!({"id": "s2", "key_name": "SPACED", "value": "two words # hash", "expires_in": 30})))
            .mount(&server).await;

        let client = ApiClient::with_tokens(
            server.uri(),
            Tokens {
                access_token: "a".into(),
                refresh_token: "r".into(),
                api_url: Some(server.uri()),
            },
        );
        let dir = std::env::temp_dir().join(format!("secrets-pull-test-{}", std::process::id()));
        std::fs::create_dir_all(&dir).unwrap();
        let file = dir.join(".env");

        execute(&client, "app", "staging", Some(file.to_str().unwrap()))
            .await
            .unwrap();

        assert_eq!(
            std::fs::read_to_string(&file).unwrap(),
            "PLAIN=abc123\nSPACED='two words # hash'\n"
        );
        #[cfg(unix)]
        {
            use std::os::unix::fs::PermissionsExt;
            assert_eq!(
                std::fs::metadata(&file).unwrap().permissions().mode() & 0o777,
                0o600
            );
        }
        std::fs::remove_dir_all(&dir).unwrap();
    }
}
