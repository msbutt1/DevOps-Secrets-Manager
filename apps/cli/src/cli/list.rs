use crate::api::ApiClient;
use crate::cli::resolve;
use crate::utils::{print_json, OutputFormat, TablePrinter};
use anyhow::Result;

/// Lists secret names and metadata in an environment. Values are never fetched.
pub async fn execute(
    client: &ApiClient,
    vault: &str,
    env_name: &str,
    output: OutputFormat,
) -> Result<()> {
    let (vault, env) = resolve::environment(client, vault, env_name).await?;
    let secrets = client.list_secrets(&env.id).await?;
    if output == OutputFormat::Json {
        return print_json(&secrets);
    }
    if secrets.is_empty() {
        println!("No secrets in {}/{}.", vault.name, env.name);
        return Ok(());
    }
    TablePrinter::print_secrets(&secrets);
    Ok(())
}
