use anyhow::Result;
use crate::api::ApiClient;
use crate::config::TokenStore;

pub async fn execute() -> Result<()> {
    // Try to call logout endpoint
    let client = ApiClient::new();
    let _ = client.logout().await;

    // Clear stored tokens
    TokenStore::clear()?;

    println!("Successfully logged out!");
    Ok(())
}
