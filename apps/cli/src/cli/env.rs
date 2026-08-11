use crate::api::ApiClient;
use crate::utils::TablePrinter;
use anyhow::{Context, Result};

pub async fn list(vault: &str) -> Result<()> {
    let client = ApiClient::new();

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
