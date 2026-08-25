use crate::api::client::{Environment, Vault};
use crate::api::ApiClient;
use anyhow::{Context, Result};

/// Finds a vault by name, or by ID when no vault has that name.
pub async fn vault(client: &ApiClient, name_or_id: &str) -> Result<Vault> {
    let vaults = client.list_vaults().await?;
    vaults
        .iter()
        .find(|v| v.name == name_or_id)
        .or_else(|| vaults.iter().find(|v| v.id == name_or_id))
        .cloned()
        .with_context(|| format!("Vault '{name_or_id}' not found (or you have no access to it)"))
}

/// Finds an environment by name inside a vault given by name or ID.
pub async fn environment(
    client: &ApiClient,
    vault_name_or_id: &str,
    env_name: &str,
) -> Result<(Vault, Environment)> {
    let vault = vault(client, vault_name_or_id).await?;
    let env = client
        .find_environment_by_name(&vault.id, env_name)
        .await?
        .with_context(|| {
            format!(
                "Environment '{env_name}' not found in vault '{}'",
                vault.name
            )
        })?;
    Ok((vault, env))
}
