use crate::api::ApiClient;
use crate::config::TokenStore;
use anyhow::Result;

pub async fn execute(client: &ApiClient) -> Result<()> {
    // Revoke the session on the server; local tokens are cleared even if that fails
    if let Err(e) = client.logout().await {
        eprintln!("Warning: could not revoke the session on the server ({e})");
    }

    TokenStore::clear()?;

    println!("Successfully logged out!");
    Ok(())
}
