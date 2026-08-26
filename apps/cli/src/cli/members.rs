use crate::api::ApiClient;
use crate::cli::resolve;
use crate::utils::{print_json, OutputFormat, TablePrinter};
use anyhow::{bail, Context, Result};

pub const ROLES: [&str; 5] = ["owner", "admin", "developer", "oncall", "viewer"];

pub async fn list(client: &ApiClient, vault: &str, output: OutputFormat) -> Result<()> {
    let vault = resolve::vault(client, vault).await?;
    let members = client.list_members(&vault.id).await?;
    if output == OutputFormat::Json {
        return print_json(&members);
    }
    if members.is_empty() {
        println!("Vault '{}' has no members.", vault.name);
        return Ok(());
    }
    TablePrinter::print_members(&members);
    Ok(())
}

pub async fn add(client: &ApiClient, vault: &str, email: &str, role: &str) -> Result<()> {
    let vault = resolve::vault(client, vault).await?;
    let member = client
        .add_member(&vault.id, email, role)
        .await
        .with_context(|| format!("Could not add {email} to vault '{}'", vault.name))?;
    println!(
        "Added {} to vault '{}' as {}.",
        member.email, vault.name, member.role
    );
    Ok(())
}

pub async fn remove(client: &ApiClient, vault: &str, email: &str) -> Result<()> {
    let vault = resolve::vault(client, vault).await?;
    let members = client.list_members(&vault.id).await?;
    let Some(member) = members
        .iter()
        .find(|m| m.email.eq_ignore_ascii_case(email.trim()))
    else {
        bail!("{email} is not a member of vault '{}'", vault.name);
    };
    client
        .remove_member(&vault.id, &member.user_id)
        .await
        .with_context(|| format!("Could not remove {email} from vault '{}'", vault.name))?;
    println!("Removed {} from vault '{}'.", member.email, vault.name);
    Ok(())
}
