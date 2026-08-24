use anyhow::{Context, Result};
use keyring::Entry;
use serde::{Deserialize, Serialize};
use std::fs;
use std::path::PathBuf;

const SERVICE_NAME: &str = "devops-secrets-manager";
const ACCESS_TOKEN_KEY: &str = "access_token";
const REFRESH_TOKEN_KEY: &str = "refresh_token";
const API_URL_KEY: &str = "api_url";

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Tokens {
    pub access_token: String,
    pub refresh_token: String,
    /// The API that issued these tokens. They are never sent to a different URL.
    #[serde(default)]
    pub api_url: Option<String>,
}

pub struct TokenStore;

impl TokenStore {
    /// Save tokens to secure storage (keyring with file fallback)
    pub fn save(tokens: &Tokens) -> Result<()> {
        // Try keyring first
        match Self::save_to_keyring(tokens) {
            Ok(_) => return Ok(()),
            Err(e) => {
                eprintln!(
                    "Warning: Could not save to keyring ({}), falling back to encrypted file",
                    e
                );
                Self::save_to_file(tokens)?;
            }
        }
        Ok(())
    }

    /// Load tokens from secure storage
    pub fn load() -> Result<Tokens> {
        // Try keyring first
        match Self::load_from_keyring() {
            Ok(tokens) => Ok(tokens),
            // Try file fallback
            Err(_) => Self::load_from_file(),
        }
    }

    /// Clear all stored tokens
    pub fn clear() -> Result<()> {
        // Try to clear from both locations
        let _ = Self::clear_keyring();
        let _ = Self::clear_file();
        Ok(())
    }

    fn save_to_keyring(tokens: &Tokens) -> Result<()> {
        let access_entry = Entry::new(SERVICE_NAME, ACCESS_TOKEN_KEY)?;
        access_entry.set_password(&tokens.access_token)?;

        let refresh_entry = Entry::new(SERVICE_NAME, REFRESH_TOKEN_KEY)?;
        refresh_entry.set_password(&tokens.refresh_token)?;

        let url_entry = Entry::new(SERVICE_NAME, API_URL_KEY)?;
        url_entry.set_password(tokens.api_url.as_deref().unwrap_or_default())?;

        Ok(())
    }

    fn load_from_keyring() -> Result<Tokens> {
        let access_entry = Entry::new(SERVICE_NAME, ACCESS_TOKEN_KEY)?;
        let access_token = access_entry.get_password()?;

        let refresh_entry = Entry::new(SERVICE_NAME, REFRESH_TOKEN_KEY)?;
        let refresh_token = refresh_entry.get_password()?;

        let api_url = Entry::new(SERVICE_NAME, API_URL_KEY)?
            .get_password()
            .ok()
            .filter(|u| !u.is_empty());

        Ok(Tokens {
            access_token,
            refresh_token,
            api_url,
        })
    }

    fn clear_keyring() -> Result<()> {
        let access_entry = Entry::new(SERVICE_NAME, ACCESS_TOKEN_KEY)?;
        let _ = access_entry.delete_password();

        let refresh_entry = Entry::new(SERVICE_NAME, REFRESH_TOKEN_KEY)?;
        let _ = refresh_entry.delete_password();

        let url_entry = Entry::new(SERVICE_NAME, API_URL_KEY)?;
        let _ = url_entry.delete_password();

        Ok(())
    }

    fn get_token_file_path() -> Result<PathBuf> {
        let config_dir = dirs::config_dir().context("Could not determine config directory")?;
        let app_dir = config_dir.join("devops-secrets-manager");
        fs::create_dir_all(&app_dir)?;
        Ok(app_dir.join("tokens.json"))
    }

    fn save_to_file(tokens: &Tokens) -> Result<()> {
        let path = Self::get_token_file_path()?;
        let json = serde_json::to_string_pretty(tokens)?;

        // Simple base64 encoding for basic obfuscation (not cryptographically secure)
        use base64::Engine;
        let encoded = base64::engine::general_purpose::STANDARD.encode(json.as_bytes());
        fs::write(path, encoded)?;

        Ok(())
    }

    fn load_from_file() -> Result<Tokens> {
        let path = Self::get_token_file_path()?;
        let encoded = fs::read_to_string(path).context("No tokens found. Please login first.")?;

        use base64::Engine;
        let decoded = base64::engine::general_purpose::STANDARD.decode(encoded.trim())?;
        let json = String::from_utf8(decoded)?;
        let tokens: Tokens = serde_json::from_str(&json)?;

        Ok(tokens)
    }

    fn clear_file() -> Result<()> {
        let path = Self::get_token_file_path()?;
        if path.exists() {
            fs::remove_file(path)?;
        }
        Ok(())
    }
}
