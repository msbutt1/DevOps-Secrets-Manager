use crate::api::ApiClient;
use crate::utils::TablePrinter;
use anyhow::Result;

pub async fn execute(vault: Option<&str>, _since: Option<&str>) -> Result<()> {
    let client = ApiClient::new();

    // Resolve vault name to ID if provided
    let vault_id = if let Some(v) = vault {
        Some(
            if let Some(vault_obj) = client.find_vault_by_name(v).await? {
                vault_obj.id
            } else {
                v.to_string()
            },
        )
    } else {
        None
    };

    let logs = client.get_audit_logs(vault_id.as_deref()).await?;

    if logs.is_empty() {
        println!("No audit logs found.");
        return Ok(());
    }

    // TODO: Filter by 'since' duration if needed
    // For now, just display all logs

    TablePrinter::print_audit_logs(&logs);
    println!("\nTotal logs: {}", logs.len());

    Ok(())
}
