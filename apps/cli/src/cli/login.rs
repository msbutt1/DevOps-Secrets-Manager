use crate::api::ApiClient;
use crate::config::{Settings, TokenStore};
use anyhow::Result;

pub async fn execute(
    client: &ApiClient,
    email_arg: Option<String>,
    password_arg: Option<String>,
    save_api_url: bool,
) -> Result<()> {
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
    println!("Authenticating with {}...", client.base_url());

    let tokens = client.login(&email, &password).await?;

    // Save tokens to secure storage
    TokenStore::save(&tokens)?;

    if save_api_url {
        let settings = Settings {
            api_url: Some(client.base_url().to_string()),
        };
        settings.save()?;
        println!("Saved {} as the default API URL.", client.base_url());
    }

    println!("Successfully logged in!");
    Ok(())
}
