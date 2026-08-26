use crate::api::ApiClient;
use crate::utils::{print_json, OutputFormat, TablePrinter};
use anyhow::{Context, Result};

pub async fn list(client: &ApiClient, output: OutputFormat) -> Result<()> {
    let vaults = client.list_vaults().await?;
    if output == OutputFormat::Json {
        return print_json(&vaults);
    }

    if vaults.is_empty() {
        println!("No vaults found.");
        return Ok(());
    }

    TablePrinter::print_vaults(&vaults);
    Ok(())
}

pub async fn create(
    client: &ApiClient,
    name: &str,
    description: Option<&str>,
    organization_id: Option<&str>,
) -> Result<()> {
    let vault = client
        .create_vault(name, description, organization_id)
        .await
        .with_context(|| format!("Failed to create vault '{name}'"))?;
    println!("Created vault '{}'. You are its owner.", vault.name);
    println!("ID: {}", vault.id);
    println!("Next: secrets env create production --vault {}", vault.name);
    Ok(())
}
