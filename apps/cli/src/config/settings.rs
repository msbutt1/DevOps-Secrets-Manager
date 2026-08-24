use anyhow::{Context, Result};
use serde::{Deserialize, Serialize};
use std::fs;
use std::path::PathBuf;

/// Used when no URL is given on the command line, in SECRETS_API_URL or in the settings file.
pub const DEFAULT_API_URL: &str = "http://localhost:8080";

/// Environment variable that overrides the saved API URL.
pub const API_URL_ENV: &str = "SECRETS_API_URL";

/// Persistent CLI settings, stored as JSON in the user's config directory.
#[derive(Debug, Default, Clone, Serialize, Deserialize, PartialEq)]
pub struct Settings {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub api_url: Option<String>,
}

impl Settings {
    fn path() -> Result<PathBuf> {
        let dir = dirs::config_dir()
            .context("Could not determine config directory")?
            .join("devops-secrets-manager");
        fs::create_dir_all(&dir)?;
        Ok(dir.join("settings.json"))
    }

    pub fn load() -> Settings {
        Self::path()
            .ok()
            .and_then(|p| fs::read_to_string(p).ok())
            .and_then(|s| serde_json::from_str(&s).ok())
            .unwrap_or_default()
    }

    pub fn save(&self) -> Result<()> {
        fs::write(Self::path()?, serde_json::to_string_pretty(self)?)?;
        Ok(())
    }
}

/// Normalises a user-supplied API URL: trims whitespace and trailing slashes and requires an
/// http or https scheme.
pub fn normalize_api_url(raw: &str) -> Result<String> {
    let url = raw.trim().trim_end_matches('/');
    let parsed = reqwest::Url::parse(url).with_context(|| format!("Invalid API URL '{raw}'"))?;
    if parsed.scheme() != "http" && parsed.scheme() != "https" {
        anyhow::bail!("API URL must start with http:// or https://");
    }
    if parsed.host_str().is_none() {
        anyhow::bail!("API URL '{raw}' has no host");
    }
    Ok(url.to_string())
}

/// Chooses the API URL: the --api-url flag, then SECRETS_API_URL, then the saved setting, then
/// the default.
pub fn resolve_api_url(
    flag: Option<&str>,
    env: Option<&str>,
    settings: &Settings,
) -> Result<String> {
    let chosen = flag
        .filter(|v| !v.trim().is_empty())
        .or(env.filter(|v| !v.trim().is_empty()))
        .or(settings.api_url.as_deref())
        .unwrap_or(DEFAULT_API_URL);
    normalize_api_url(chosen)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn precedence_is_flag_then_env_then_saved_then_default() {
        let saved = Settings {
            api_url: Some("https://saved.example".into()),
        };
        assert_eq!(
            resolve_api_url(
                Some("https://flag.example/"),
                Some("https://env.example"),
                &saved
            )
            .unwrap(),
            "https://flag.example"
        );
        assert_eq!(
            resolve_api_url(None, Some("https://env.example"), &saved).unwrap(),
            "https://env.example"
        );
        assert_eq!(
            resolve_api_url(None, Some("  "), &saved).unwrap(),
            "https://saved.example"
        );
        assert_eq!(
            resolve_api_url(None, None, &Settings::default()).unwrap(),
            DEFAULT_API_URL
        );
    }

    #[test]
    fn rejects_urls_without_http_scheme() {
        assert!(normalize_api_url("ftp://example.com").is_err());
        assert!(normalize_api_url("example.com").is_err());
        assert_eq!(
            normalize_api_url(" http://localhost:8080/// ").unwrap(),
            "http://localhost:8080"
        );
    }
}
