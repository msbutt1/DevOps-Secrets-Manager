use crate::api::ApiClient;
use crate::cli::resolve;
use anyhow::{bail, Context, Result};
use std::io::{self, BufRead, IsTerminal, Write};

pub async fn execute(
    client: &ApiClient,
    key_name: &str,
    vault: &str,
    env_name: &str,
    yes: bool,
) -> Result<()> {
    let (vault, env) = resolve::environment(client, vault, env_name).await?;
    let secret = client
        .list_secrets(&env.id)
        .await?
        .into_iter()
        .find(|s| s.key_name == key_name)
        .with_context(|| {
            format!(
                "Secret '{key_name}' not found in {}/{}",
                vault.name, env.name
            )
        })?;

    if !yes {
        if !io::stdin().is_terminal() {
            bail!("Refusing to delete without confirmation; pass --yes in scripts");
        }
        print!(
            "Delete {} from {}/{}? Type the key name to confirm: ",
            key_name, vault.name, env.name
        );
        io::stdout().flush()?;
        let mut answer = String::new();
        io::stdin().lock().read_line(&mut answer)?;
        if answer.trim() != key_name {
            bail!("Not deleted: the name did not match");
        }
    }

    client.delete_secret(&secret.id).await?;
    println!("Deleted {} from {}/{}.", key_name, vault.name, env.name);
    Ok(())
}
