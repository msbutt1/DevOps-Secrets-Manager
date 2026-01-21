use anyhow::{Result, Context};
use crate::api::ApiClient;
use std::fs;

pub async fn execute(vault: &str, env: &str, out: Option<&str>) -> Result<()> {
    let client = ApiClient::new();

    // Resolve vault name to ID
    let vault_id = if let Some(v) = client.find_vault_by_name(vault).await? {
        v.id
    } else {
        vault.to_string()
    };

    // Resolve environment name to ID
    let env_obj = client.find_environment_by_name(&vault_id, env).await?
        .context(format!("Environment '{}' not found in vault '{}'", env, vault))?;

    // Get all secrets
    let secrets = client.list_secrets(&env_obj.id).await?;

    if secrets.is_empty() {
        println!("No secrets found in environment '{}/{}'.", vault, env);
        return Ok(());
    }

    // Reveal all secret values
    let mut output = String::new();
    for secret in &secrets {
        let value = client.reveal_secret(&secret.id).await
            .context(format!("Failed to reveal secret '{}'", secret.key_name))?;
        output.push_str(&format!("{}={}\n", secret.key_name, value));
    }

    // Write to file or stdout
    if let Some(file_path) = out {
        fs::write(file_path, output)?;
        println!("Secrets written to {}", file_path);
    } else {
        print!("{}", output);
    }

    Ok(())
}
