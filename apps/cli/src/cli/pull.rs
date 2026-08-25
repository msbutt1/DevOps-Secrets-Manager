use crate::api::ApiClient;
use crate::utils::{dotenv, write_private};
use anyhow::{Context, Result};
use std::path::Path;

pub async fn execute(client: &ApiClient, vault: &str, env: &str, out: Option<&str>) -> Result<()> {
    // Resolve vault name to ID
    let vault_id = if let Some(v) = client.find_vault_by_name(vault).await? {
        v.id
    } else {
        vault.to_string()
    };

    // Resolve environment name to ID
    let env_obj = client
        .find_environment_by_name(&vault_id, env)
        .await?
        .context(format!(
            "Environment '{}' not found in vault '{}'",
            env, vault
        ))?;

    // Get all secrets
    let secrets = client.list_secrets(&env_obj.id).await?;

    if secrets.is_empty() {
        println!("No secrets found in environment '{}/{}'.", vault, env);
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
