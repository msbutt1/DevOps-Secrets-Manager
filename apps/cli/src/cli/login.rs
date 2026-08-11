use crate::api::ApiClient;
use crate::config::TokenStore;
use anyhow::Result;

pub async fn execute(email_arg: Option<String>, password_arg: Option<String>) -> Result<()> {
    println!("DevOps Secrets Manager - Login");
    println!();

    let email = if let Some(e) = email_arg {
        e
    } else {
        print!("Email: ");
        use std::io::{self, Write};
        io::stdout().flush()?;

        let mut email = String::new();
        io::stdin().read_line(&mut email)?;
        email.trim().to_string()
    };

    let password = if let Some(p) = password_arg {
        p
    } else {
        rpassword::prompt_password("Password: ")?
    };

    println!();
    println!("Authenticating...");

    let client = ApiClient::new();
    let tokens = client.login(&email, &password).await?;

    // Save tokens to secure storage
    TokenStore::save(&tokens)?;

    println!("Successfully logged in!");
    Ok(())
}
