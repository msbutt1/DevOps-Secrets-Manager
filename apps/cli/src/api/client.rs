use anyhow::{Context, Result};
use reqwest::{Client, StatusCode};
use serde::{Deserialize, Serialize};
use std::sync::Arc;
use tokio::sync::Mutex;
use thiserror::Error;

use crate::config::{TokenStore, Tokens};
use tokio::sync::MutexGuard;

const API_BASE_URL: &str = "http://localhost:8080";

#[derive(Error, Debug)]
pub enum ApiError {
    #[error("Authentication failed: {0}")]
    AuthError(String),

    #[error("Not found: {0}")]
    NotFound(String),

    #[error("API error: {0}")]
    ApiError(String),

    #[error("Network error: {0}")]
    NetworkError(#[from] reqwest::Error),

    #[error("Other error: {0}")]
    Other(#[from] anyhow::Error),
}

// Request/Response structs
#[derive(Serialize)]
pub struct LoginRequest {
    pub email: String,
    pub password: String,
}

#[derive(Deserialize)]
pub struct LoginResponse {
    pub access_token: String,
    pub refresh_token: String,
}

#[derive(Serialize)]
pub struct RefreshRequest {
    pub refresh_token: String,
}

#[derive(Deserialize)]
pub struct RefreshResponse {
    pub access_token: String,
    pub refresh_token: String,
}

#[derive(Deserialize, Debug, Clone)]
pub struct Vault {
    pub id: String,
    pub name: String,
    pub description: Option<String>,
    pub created_at: String,
}

#[derive(Deserialize, Debug, Clone)]
pub struct Environment {
    pub id: String,
    pub name: String,
    pub vault_id: String,
    pub description: Option<String>,
}

#[derive(Deserialize, Debug, Clone)]
pub struct Secret {
    pub id: String,
    pub key_name: String,
    pub description: Option<String>,
    pub environment_id: String,
    pub rotation_interval_days: Option<i32>,
    pub last_rotated_at: Option<String>,
    pub expires_at: Option<String>,
    pub created_at: String,
}

#[derive(Serialize)]
pub struct CreateSecretRequest {
    pub key_name: String,
    pub value: String,
    pub description: Option<String>,
    pub rotation_interval_days: Option<i32>,
    pub expires_at: Option<String>,
    pub metadata: Option<serde_json::Value>,
}

#[derive(Deserialize)]
pub struct RevealSecretResponse {
    pub value: String,
}

#[derive(Deserialize, Debug)]
pub struct AuditLog {
    pub id: String,
    pub action: String,
    pub user_id: String,
    pub vault_id: Option<String>,
    pub environment_id: Option<String>,
    pub secret_id: Option<String>,
    pub ip_address: Option<String>,
    pub user_agent: Option<String>,
    pub timestamp: String,
}

pub struct ApiClient {
    client: Client,
    tokens: Arc<Mutex<Option<Tokens>>>,
}

impl ApiClient {
    pub fn new() -> Self {
        Self {
            client: Client::new(),
            tokens: Arc::new(Mutex::new(None)),
        }
    }

    pub async fn login(&self, email: &str, password: &str) -> Result<Tokens, ApiError> {
        let request = LoginRequest {
            email: email.to_string(),
            password: password.to_string(),
        };

        let response = self
            .client
            .post(format!("{}/auth/login", API_BASE_URL))
            .json(&request)
            .send()
            .await?;

        if !response.status().is_success() {
            let status = response.status();
            let body = response.text().await.unwrap_or_default();
            return Err(ApiError::AuthError(format!(
                "Login failed with status {}: {}",
                status, body
            )));
        }

        let login_response: LoginResponse = response.json().await?;
        let tokens = Tokens {
            access_token: login_response.access_token,
            refresh_token: login_response.refresh_token,
        };

        // Store tokens in memory
        *self.tokens.lock().await = Some(tokens.clone());

        Ok(tokens)
    }

    pub async fn logout(&self) -> Result<(), ApiError> {
        let tokens_guard: MutexGuard<Option<Tokens>> = self.tokens.lock().await;
        if let Some(tokens) = tokens_guard.as_ref() {
            let _ = self
                .client
                .post(format!("{}/auth/logout", API_BASE_URL))
                .bearer_auth(&tokens.access_token)
                .send()
                .await;
        }
        Ok(())
    }

    async fn refresh_tokens(&self) -> Result<(), ApiError> {
        let mut tokens_guard: MutexGuard<Option<Tokens>> = self.tokens.lock().await;

        let current_tokens = tokens_guard
            .as_ref()
            .ok_or_else(|| ApiError::AuthError("No tokens available".to_string()))?;

        let request = RefreshRequest {
            refresh_token: current_tokens.refresh_token.clone(),
        };

        let response = self
            .client
            .post(format!("{}/auth/refresh", API_BASE_URL))
            .json(&request)
            .send()
            .await?;

        if !response.status().is_success() {
            return Err(ApiError::AuthError(
                "Token refresh failed. Please login again.".to_string(),
            ));
        }

        let refresh_response: RefreshResponse = response.json().await?;
        let new_tokens = Tokens {
            access_token: refresh_response.access_token,
            refresh_token: refresh_response.refresh_token,
        };

        // Save new tokens
        TokenStore::save(&new_tokens)?;
        *tokens_guard = Some(new_tokens);

        Ok(())
    }

    async fn get_with_auth(&self, url: &str) -> Result<reqwest::Response, ApiError> {
        self.request_with_auth("GET", url, None::<&()>).await
    }

    async fn post_with_auth<T: Serialize>(
        &self,
        url: &str,
        body: &T,
    ) -> Result<reqwest::Response, ApiError> {
        self.request_with_auth("POST", url, Some(body)).await
    }

    async fn request_with_auth<T: Serialize>(
        &self,
        method: &str,
        url: &str,
        body: Option<&T>,
    ) -> Result<reqwest::Response, ApiError> {
        // Load tokens if not in memory
        {
            let tokens_guard: MutexGuard<Option<Tokens>> = self.tokens.lock().await;
            if tokens_guard.is_none() {
                drop(tokens_guard);
                let stored_tokens = TokenStore::load()
                    .context("Not logged in. Please run 'secrets login' first.")?;
                *self.tokens.lock().await = Some(stored_tokens);
            }
        }

        let tokens_guard: MutexGuard<Option<Tokens>> = self.tokens.lock().await;
        let tokens = tokens_guard
            .as_ref()
            .ok_or_else(|| ApiError::AuthError("No tokens available".to_string()))?;

        let mut request = match method {
            "GET" => self.client.get(url),
            "POST" => self.client.post(url),
            _ => return Err(ApiError::Other(anyhow::anyhow!("Unsupported method"))),
        };

        request = request.bearer_auth(&tokens.access_token);

        if let Some(body) = body {
            request = request.json(body);
        }

        drop(tokens_guard);

        let response = request.send().await?;

        // Handle 401 by refreshing token and retrying
        if response.status() == StatusCode::UNAUTHORIZED {
            drop(response);
            self.refresh_tokens().await?;

            // Retry the request
            let tokens_guard: MutexGuard<Option<Tokens>> = self.tokens.lock().await;
            let tokens = tokens_guard.as_ref().unwrap();

            let mut retry_request = match method {
                "GET" => self.client.get(url),
                "POST" => self.client.post(url),
                _ => return Err(ApiError::Other(anyhow::anyhow!("Unsupported method"))),
            };

            retry_request = retry_request.bearer_auth(&tokens.access_token);

            if let Some(body) = body {
                retry_request = retry_request.json(body);
            }

            let retry_response = retry_request.send().await?;
            return Ok(retry_response);
        }

        Ok(response)
    }

    pub async fn list_vaults(&self) -> Result<Vec<Vault>, ApiError> {
        let response = self
            .get_with_auth(&format!("{}/vaults", API_BASE_URL))
            .await?;

        if !response.status().is_success() {
            let status = response.status();
            let body = response.text().await.unwrap_or_default();
            return Err(ApiError::ApiError(format!(
                "Failed to list vaults: {} - {}",
                status, body
            )));
        }

        let vaults: Vec<Vault> = response.json().await?;
        Ok(vaults)
    }

    pub async fn list_environments(&self, vault_id: &str) -> Result<Vec<Environment>, ApiError> {
        let response = self
            .get_with_auth(&format!("{}/vaults/{}/envs", API_BASE_URL, vault_id))
            .await?;

        if !response.status().is_success() {
            let status = response.status();
            let body = response.text().await.unwrap_or_default();
            return Err(ApiError::ApiError(format!(
                "Failed to list environments: {} - {}",
                status, body
            )));
        }

        let environments: Vec<Environment> = response.json().await?;
        Ok(environments)
    }

    pub async fn list_secrets(&self, env_id: &str) -> Result<Vec<Secret>, ApiError> {
        let response = self
            .get_with_auth(&format!("{}/envs/{}/secrets", API_BASE_URL, env_id))
            .await?;

        if !response.status().is_success() {
            let status = response.status();
            let body = response.text().await.unwrap_or_default();
            return Err(ApiError::ApiError(format!(
                "Failed to list secrets: {} - {}",
                status, body
            )));
        }

        let secrets: Vec<Secret> = response.json().await?;
        Ok(secrets)
    }

    pub async fn reveal_secret(&self, secret_id: &str) -> Result<String, ApiError> {
        let response = self
            .post_with_auth::<()>(
                &format!("{}/secrets/{}/reveal", API_BASE_URL, secret_id),
                &(),
            )
            .await?;

        if !response.status().is_success() {
            let status = response.status();
            let body = response.text().await.unwrap_or_default();
            return Err(ApiError::ApiError(format!(
                "Failed to reveal secret: {} - {}",
                status, body
            )));
        }

        let reveal_response: RevealSecretResponse = response.json().await?;
        Ok(reveal_response.value)
    }

    pub async fn create_secret(
        &self,
        env_id: &str,
        request: CreateSecretRequest,
    ) -> Result<Secret, ApiError> {
        let response = self
            .post_with_auth(&format!("{}/envs/{}/secrets", API_BASE_URL, env_id), &request)
            .await?;

        if !response.status().is_success() {
            let status = response.status();
            let body = response.text().await.unwrap_or_default();
            return Err(ApiError::ApiError(format!(
                "Failed to create secret: {} - {}",
                status, body
            )));
        }

        let secret: Secret = response.json().await?;
        Ok(secret)
    }

    pub async fn get_audit_logs(&self, vault_id: Option<&str>) -> Result<Vec<AuditLog>, ApiError> {
        let mut url = format!("{}/audit", API_BASE_URL);
        if let Some(vid) = vault_id {
            url.push_str(&format!("?vault_id={}", vid));
        }

        let response = self.get_with_auth(&url).await?;

        if !response.status().is_success() {
            let status = response.status();
            let body = response.text().await.unwrap_or_default();
            return Err(ApiError::ApiError(format!(
                "Failed to get audit logs: {} - {}",
                status, body
            )));
        }

        let logs: Vec<AuditLog> = response.json().await?;
        Ok(logs)
    }

    pub async fn find_vault_by_name(&self, name: &str) -> Result<Option<Vault>, ApiError> {
        let vaults = self.list_vaults().await?;
        Ok(vaults.into_iter().find(|v| v.name == name))
    }

    pub async fn find_environment_by_name(
        &self,
        vault_id: &str,
        name: &str,
    ) -> Result<Option<Environment>, ApiError> {
        let environments = self.list_environments(vault_id).await?;
        Ok(environments.into_iter().find(|e| e.name == name))
    }
}
