use anyhow::{Result, Context, bail};
use crate::api::{ApiClient, client::CreateSecretRequest};

pub async fn execute(
    secret: &str,
    vault: &str,
    env_name: &str,
    description: Option<&str>,
    rotation_days: Option<i32>,
) -> Result<()> {
    // Parse KEY=value format
    let parts: Vec<&str> = secret.splitn(2, '=').collect();
    if parts.len() != 2 {
        bail!("Invalid format. Expected KEY=value");
    }

    let key_name = parts[0].trim();
    let value = parts[1];

    if key_name.is_empty() {
        bail!("Key name cannot be empty");
    }

    let client = ApiClient::new();

    // Resolve vault name to ID
    let vault_id = if let Some(v) = client.find_vault_by_name(vault).await? {
        v.id
    } else {
        vault.to_string()
    };

    // Resolve environment name to ID
    let env_obj = client.find_environment_by_name(&vault_id, env_name).await?
        .context(format!("Environment '{}' not found in vault '{}'", env_name, vault))?;

    // Create secret
    let request = CreateSecretRequest {
        key_name: key_name.to_string(),
        value: value.to_string(),
        description: description.map(String::from),
        rotation_interval_days: rotation_days,
        expires_at: None,
        metadata: None,
    };

    let created_secret = client.create_secret(&env_obj.id, request).await?;

    println!("Secret '{}' created successfully!", created_secret.key_name);
    println!("ID: {}", created_secret.id);

    Ok(())
}
