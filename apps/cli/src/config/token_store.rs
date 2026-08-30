use anyhow::{Context, Result};
use keyring::Entry;
use serde::{Deserialize, Serialize};
use std::fs;
use std::path::{Path, PathBuf};

use crate::utils::write_private;

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
                    "Warning: Could not save to keyring ({}); saving to a file only you can read",
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
        Self::save_to_path(&Self::get_token_file_path()?, tokens)
    }

    fn load_from_file() -> Result<Tokens> {
        Self::load_from_path(&Self::get_token_file_path()?)
    }

    /// Writes the tokens as JSON readable only by the current user (mode 0600). The file is
    /// not encrypted, so its protection is the file permission, like `~/.ssh` keys.
    fn save_to_path(path: &Path, tokens: &Tokens) -> Result<()> {
        let json = serde_json::to_string_pretty(tokens)?;
        write_private(path, &json)
    }

    fn load_from_path(path: &Path) -> Result<Tokens> {
        let contents = fs::read_to_string(path).context("No tokens found. Please login first.")?;
        if let Ok(tokens) = serde_json::from_str(&contents) {
            return Ok(tokens);
        }

        // Older versions wrote base64-encoded JSON
        use base64::Engine;
        let decoded = base64::engine::general_purpose::STANDARD.decode(contents.trim())?;
        let json = String::from_utf8(decoded)?;
        Ok(serde_json::from_str(&json)?)
    }

    fn clear_file() -> Result<()> {
        let path = Self::get_token_file_path()?;
        if path.exists() {
            fs::remove_file(path)?;
        }
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn temp_dir(name: &str) -> PathBuf {
        let dir =
            std::env::temp_dir().join(format!("secrets-cli-token-{}-{}", name, std::process::id()));
        fs::create_dir_all(&dir).unwrap();
        dir
    }

    fn sample() -> Tokens {
        Tokens {
            access_token: "access".into(),
            refresh_token: "refresh".into(),
            api_url: Some("http://localhost:8080".into()),
        }
    }

    #[cfg(unix)]
    #[test]
    fn token_file_is_private() {
        use std::os::unix::fs::PermissionsExt;
        let dir = temp_dir("private");
        let path = dir.join("tokens.json");
        // A file left behind by an older version with default permissions
        fs::write(&path, "old").unwrap();
        fs::set_permissions(&path, fs::Permissions::from_mode(0o644)).unwrap();

        TokenStore::save_to_path(&path, &sample()).unwrap();

        assert_eq!(
            fs::metadata(&path).unwrap().permissions().mode() & 0o777,
            0o600
        );
        let loaded = TokenStore::load_from_path(&path).unwrap();
        assert_eq!(loaded.refresh_token, "refresh");
        assert_eq!(loaded.api_url.as_deref(), Some("http://localhost:8080"));
        fs::remove_dir_all(&dir).unwrap();
    }

    #[test]
    fn reads_files_written_by_older_versions() {
        use base64::Engine;
        let dir = temp_dir("legacy");
        let path = dir.join("tokens.json");
        let json = serde_json::to_string(&sample()).unwrap();
        fs::write(
            &path,
            base64::engine::general_purpose::STANDARD.encode(json.as_bytes()),
        )
        .unwrap();

        let loaded = TokenStore::load_from_path(&path).unwrap();
        assert_eq!(loaded.access_token, "access");
        fs::remove_dir_all(&dir).unwrap();
    }
}
