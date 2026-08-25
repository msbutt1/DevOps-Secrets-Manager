use crate::api::ApiClient;
use crate::cli::resolve;
use crate::utils::TablePrinter;
use anyhow::{Context, Result};

pub async fn list(client: &ApiClient, vault: &str) -> Result<()> {
    // Try to find vault by name first, then use as ID
    let vault_id = if let Some(v) = client.find_vault_by_name(vault).await? {
        v.id
    } else {
        vault.to_string()
    };

    let environments = client
        .list_environments(&vault_id)
        .await
        .context(format!("Failed to list environments for vault '{}'", vault))?;

    if environments.is_empty() {
        println!("No environments found in vault '{}'.", vault);
        return Ok(());
    }

    TablePrinter::print_environments(&environments);
    Ok(())
}

pub async fn create(
    client: &ApiClient,
    vault: &str,
    name: &str,
    description: Option<&str>,
) -> Result<()> {
    let vault = resolve::vault(client, vault).await?;
    let env = client
        .create_environment(&vault.id, name, description)
        .await
        .with_context(|| {
            format!(
                "Failed to create environment '{name}' in vault '{}'",
                vault.name
            )
        })?;
    println!(
        "Created environment '{}' in vault '{}'.",
        env.name, vault.name
    );
    println!("ID: {}", env.id);
    Ok(())
}
