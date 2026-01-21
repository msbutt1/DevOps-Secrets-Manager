use anyhow::Result;
use crate::api::ApiClient;
use crate::utils::TablePrinter;

pub async fn list() -> Result<()> {
    let client = ApiClient::new();
    let vaults = client.list_vaults().await?;

    if vaults.is_empty() {
        println!("No vaults found.");
        return Ok(());
    }

    TablePrinter::print_vaults(&vaults);
    Ok(())
}
