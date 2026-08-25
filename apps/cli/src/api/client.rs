use reqwest::{Client, Method, StatusCode};
use serde::de::DeserializeOwned;
use serde::{Deserialize, Serialize};
use thiserror::Error;
use tokio::sync::Mutex;

use crate::config::{TokenStore, Tokens};

#[derive(Error, Debug)]
pub enum ApiError {
    #[error("Authentication failed: {0}")]
    AuthError(String),

    #[error("{message} (HTTP {status})")]
    Api {
        status: u16,
        code: String,
        message: String,
    },

    #[error("Network error: {0}")]
    NetworkError(#[from] reqwest::Error),

    #[error("Other error: {0}")]
    Other(#[from] anyhow::Error),
}

/// The API's error body: `{"error": "not_found", "message": "Vault not found"}`.
#[derive(Deserialize)]
struct ErrorBody {
    #[serde(default)]
    error: String,
    #[serde(default)]
    message: String,
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
    #[serde(default)]
    pub description: Option<String>,
    #[serde(default)]
    pub rotation_interval_days: Option<i32>,
    #[serde(default)]
    pub expires_at: Option<String>,
    #[serde(default)]
    pub metadata: Option<serde_json::Value>,
}

/// Body for PUT /secrets/{id}. The API replaces description, rotation interval, expiry and
/// metadata with what is sent, so callers pass the current values for anything unchanged. A
/// missing value keeps the stored secret value.
#[derive(Serialize, Debug, Default)]
pub struct UpdateSecretRequest {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub value: Option<String>,
    pub description: Option<String>,
    pub rotation_interval_days: Option<i32>,
    pub expires_at: Option<String>,
    pub metadata: Option<serde_json::Value>,
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
    pub action: String,
    pub user_email: String,
    pub vault_name: Option<String>,
    pub environment_name: Option<String>,
    pub target_name: Option<String>,
    pub ip_address: Option<String>,
    pub timestamp: String,
}

#[derive(Deserialize, Debug)]
pub struct AuditPage {
    pub data: Vec<AuditLog>,
    pub total: u64,
}

pub struct ApiClient {
    client: Client,
    base_url: String,
    tokens: Mutex<Option<Tokens>>,
    /// Whether refreshed tokens are written to the token store (off for injected test tokens).
    persist_tokens: bool,
}

impl ApiClient {
    /// Creates a client for the API at `base_url` (already normalised, without a trailing slash).
    pub fn new(base_url: impl Into<String>) -> Self {
        Self {
            client: Client::new(),
            base_url: base_url.into(),
            tokens: Mutex::new(None),
            persist_tokens: true,
        }
    }

    /// Creates a client that uses the given tokens instead of the token store.
    #[cfg(test)]
    pub fn with_tokens(base_url: impl Into<String>, tokens: Tokens) -> Self {
        Self {
            client: Client::new(),
            base_url: base_url.into(),
            tokens: Mutex::new(Some(tokens)),
            persist_tokens: false,
        }
    }

    pub fn base_url(&self) -> &str {
        &self.base_url
    }

    fn url(&self, path: &str) -> String {
        format!("{}{}", self.base_url, path)
    }

    async fn error_from(response: reqwest::Response) -> ApiError {
        let status = response.status().as_u16();
        let body = response.text().await.unwrap_or_default();
        match serde_json::from_str::<ErrorBody>(&body) {
            Ok(e) if !e.message.is_empty() => ApiError::Api {
                status,
                code: e.error,
                message: e.message,
            },
            _ => ApiError::Api {
                status,
                code: String::new(),
                message: if body.is_empty() {
                    "Request failed".to_string()
                } else {
                    body
                },
            },
        }
    }

    pub async fn login(&self, email: &str, password: &str) -> Result<Tokens, ApiError> {
        let request = LoginRequest {
            email: email.to_string(),
            password: password.to_string(),
        };

        let response = self
            .client
            .post(self.url("/auth/login"))
            .json(&request)
            .send()
            .await?;

        if !response.status().is_success() {
            return Err(match Self::error_from(response).await {
                ApiError::Api { message, .. } => ApiError::AuthError(message),
                other => other,
            });
        }

        let login_response: LoginResponse = response.json().await?;
        let tokens = Tokens {
            access_token: login_response.access_token,
            refresh_token: login_response.refresh_token,
            api_url: Some(self.base_url.clone()),
        };

        // Store tokens in memory
        *self.tokens.lock().await = Some(tokens.clone());

        Ok(tokens)
    }

    /// Revokes the stored refresh token on the server. Errors are ignored by callers: the local
    /// tokens are cleared either way.
    pub async fn logout(&self) -> Result<(), ApiError> {
        let tokens = self.session_tokens().await?;
        let response = self
            .client
            .post(self.url("/auth/logout"))
            .json(&RefreshRequest {
                refresh_token: tokens.refresh_token,
            })
            .send()
            .await?;
        if !response.status().is_success() {
            return Err(Self::error_from(response).await);
        }
        Ok(())
    }

    /// Loads session tokens for this API, refusing tokens issued by a different API URL.
    async fn session_tokens(&self) -> Result<Tokens, ApiError> {
        let mut guard = self.tokens.lock().await;
        if guard.is_none() {
            let stored = TokenStore::load().map_err(|_| {
                ApiError::AuthError("Not logged in. Please run 'secrets login' first.".to_string())
            })?;
            *guard = Some(stored);
        }
        let tokens = guard.clone().expect("tokens loaded above");
        if let Some(issuer) = &tokens.api_url {
            if issuer != &self.base_url {
                return Err(ApiError::AuthError(format!(
                    "You are logged in to {issuer}, not {}. Run 'secrets login --api-url {}' first.",
                    self.base_url, self.base_url
                )));
            }
        }
        Ok(tokens)
    }

    async fn refresh_tokens(&self, current: &Tokens) -> Result<Tokens, ApiError> {
        let response = self
            .client
            .post(self.url("/auth/refresh"))
            .json(&RefreshRequest {
                refresh_token: current.refresh_token.clone(),
            })
            .send()
            .await?;

        if !response.status().is_success() {
            return Err(ApiError::AuthError(
                "Your session has expired. Please run 'secrets login' again.".to_string(),
            ));
        }

        let refreshed: LoginResponse = response.json().await?;
        let new_tokens = Tokens {
            access_token: refreshed.access_token,
            refresh_token: refreshed.refresh_token,
            api_url: Some(self.base_url.clone()),
        };

        // Save new tokens; a failure to persist should not fail the command
        if self.persist_tokens {
            let _ = TokenStore::save(&new_tokens);
        }
        *self.tokens.lock().await = Some(new_tokens.clone());

        Ok(new_tokens)
    }

    /// Sends an authenticated request, refreshing the session once on 401, and returns the
    /// successful response.
    async fn send<B: Serialize + ?Sized>(
        &self,
        method: Method,
        path: &str,
        body: Option<&B>,
    ) -> Result<reqwest::Response, ApiError> {
        let mut tokens = self.session_tokens().await?;

        for attempt in 0..2 {
            let mut request = self
                .client
                .request(method.clone(), self.url(path))
                .bearer_auth(&tokens.access_token);
            if let Some(body) = body {
                request = request.json(body);
            }
            let response = request.send().await?;

            if response.status() == StatusCode::UNAUTHORIZED && attempt == 0 {
                tokens = self.refresh_tokens(&tokens).await?;
                continue;
            }
            if !response.status().is_success() {
                return Err(Self::error_from(response).await);
            }
            return Ok(response);
        }
        unreachable!("the loop returns on the second attempt")
    }

    async fn get<T: DeserializeOwned>(&self, path: &str) -> Result<T, ApiError> {
        Ok(self
            .send::<()>(Method::GET, path, None)
            .await?
            .json()
            .await?)
    }

    async fn post<B: Serialize + ?Sized, T: DeserializeOwned>(
        &self,
        path: &str,
        body: &B,
    ) -> Result<T, ApiError> {
        Ok(self
            .send(Method::POST, path, Some(body))
            .await?
            .json()
            .await?)
    }

    pub async fn list_vaults(&self) -> Result<Vec<Vault>, ApiError> {
        self.get("/vaults").await
    }

    pub async fn list_environments(&self, vault_id: &str) -> Result<Vec<Environment>, ApiError> {
        self.get(&format!("/vaults/{vault_id}/envs")).await
    }

    pub async fn list_secrets(&self, env_id: &str) -> Result<Vec<Secret>, ApiError> {
        self.get(&format!("/envs/{env_id}/secrets")).await
    }

    pub async fn reveal_secret(&self, secret_id: &str) -> Result<String, ApiError> {
        let response: RevealSecretResponse = self
            .post(
                &format!("/secrets/{secret_id}/reveal"),
                &serde_json::json!({}),
            )
            .await?;
        Ok(response.value)
    }

    pub async fn create_secret(
        &self,
        env_id: &str,
        request: CreateSecretRequest,
    ) -> Result<Secret, ApiError> {
        self.post(&format!("/envs/{env_id}/secrets"), &request)
            .await
    }

    pub async fn update_secret(
        &self,
        secret_id: &str,
        request: &UpdateSecretRequest,
    ) -> Result<Secret, ApiError> {
        Ok(self
            .send(Method::PUT, &format!("/secrets/{secret_id}"), Some(request))
            .await?
            .json()
            .await?)
    }

    pub async fn create_environment(
        &self,
        vault_id: &str,
        name: &str,
        description: Option<&str>,
    ) -> Result<Environment, ApiError> {
        self.post(
            &format!("/vaults/{vault_id}/envs"),
            &serde_json::json!({"name": name, "description": description}),
        )
        .await
    }

    pub async fn delete_secret(&self, secret_id: &str) -> Result<(), ApiError> {
        self.send::<()>(Method::DELETE, &format!("/secrets/{secret_id}"), None)
            .await?;
        Ok(())
    }

    pub async fn get_audit_logs(
        &self,
        vault_id: Option<&str>,
        start_date: Option<&str>,
    ) -> Result<AuditPage, ApiError> {
        let mut url = reqwest::Url::parse("http://placeholder/audit")
            .map_err(|e| ApiError::Other(e.into()))?;
        {
            let mut query = url.query_pairs_mut();
            query.append_pair("limit", "200");
            if let Some(vid) = vault_id {
                query.append_pair("vaultId", vid);
            }
            if let Some(start) = start_date {
                query.append_pair("startDate", start);
            }
        }
        let path = format!("/audit?{}", url.query().unwrap_or_default());
        self.get(&path).await
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

#[cfg(test)]
mod tests {
    use super::*;
    use wiremock::matchers::{body_json, header, method, path};
    use wiremock::{Mock, MockServer, ResponseTemplate};

    fn tokens_for(url: &str) -> Tokens {
        Tokens {
            access_token: "access-1".into(),
            refresh_token: "refresh-1".into(),
            api_url: Some(url.to_string()),
        }
    }

    #[tokio::test]
    async fn requests_go_to_the_configured_base_url_with_the_access_token() {
        let server = MockServer::start().await;
        Mock::given(method("GET"))
            .and(path("/vaults"))
            .and(header("authorization", "Bearer access-1"))
            .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!([
                {"id": "v1", "name": "payments", "description": null, "created_at": "2026-01-01T00:00:00Z"}
            ])))
            .expect(1)
            .mount(&server)
            .await;

        let client = ApiClient::with_tokens(server.uri(), tokens_for(&server.uri()));
        let vaults = client.list_vaults().await.unwrap();
        assert_eq!(vaults[0].name, "payments");
    }

    #[tokio::test]
    async fn refuses_to_send_tokens_issued_by_another_api() {
        let server = MockServer::start().await;
        Mock::given(method("GET"))
            .respond_with(ResponseTemplate::new(200))
            .expect(0)
            .mount(&server)
            .await;

        let client = ApiClient::with_tokens(server.uri(), tokens_for("https://other.example"));
        let err = client.list_vaults().await.unwrap_err();
        assert!(
            err.to_string()
                .contains("logged in to https://other.example"),
            "{err}"
        );
    }

    #[tokio::test]
    async fn refreshes_once_on_401_and_retries() {
        let server = MockServer::start().await;
        Mock::given(method("GET"))
            .and(path("/vaults"))
            .and(header("authorization", "Bearer access-1"))
            .respond_with(ResponseTemplate::new(401))
            .expect(1)
            .mount(&server)
            .await;
        Mock::given(method("POST"))
            .and(path("/auth/refresh"))
            .and(body_json(serde_json::json!({"refresh_token": "refresh-1"})))
            .respond_with(ResponseTemplate::new(200).set_body_json(
                serde_json::json!({"access_token": "access-2", "refresh_token": "refresh-2"}),
            ))
            .expect(1)
            .mount(&server)
            .await;
        Mock::given(method("GET"))
            .and(path("/vaults"))
            .and(header("authorization", "Bearer access-2"))
            .respond_with(ResponseTemplate::new(200).set_body_json(serde_json::json!([])))
            .expect(1)
            .mount(&server)
            .await;

        let client = ApiClient::with_tokens(server.uri(), tokens_for(&server.uri()));
        assert!(client.list_vaults().await.unwrap().is_empty());
    }

    #[tokio::test]
    async fn api_error_messages_are_surfaced() {
        let server = MockServer::start().await;
        Mock::given(method("GET"))
            .and(path("/vaults/v1/envs"))
            .respond_with(ResponseTemplate::new(404).set_body_json(
                serde_json::json!({"error": "not_found", "message": "Vault not found"}),
            ))
            .mount(&server)
            .await;

        let client = ApiClient::with_tokens(server.uri(), tokens_for(&server.uri()));
        let err = client.list_environments("v1").await.unwrap_err();
        assert_eq!(err.to_string(), "Vault not found (HTTP 404)");
    }

    #[tokio::test]
    async fn delete_secret_sends_delete() {
        let server = MockServer::start().await;
        Mock::given(method("DELETE"))
            .and(path("/secrets/s1"))
            .respond_with(ResponseTemplate::new(204))
            .expect(1)
            .mount(&server)
            .await;
        let client = ApiClient::with_tokens(server.uri(), tokens_for(&server.uri()));
        client.delete_secret("s1").await.unwrap();
    }

    #[tokio::test]
    async fn logout_revokes_the_refresh_token() {
        let server = MockServer::start().await;
        Mock::given(method("POST"))
            .and(path("/auth/logout"))
            .and(body_json(serde_json::json!({"refresh_token": "refresh-1"})))
            .respond_with(ResponseTemplate::new(204))
            .expect(1)
            .mount(&server)
            .await;

        let client = ApiClient::with_tokens(server.uri(), tokens_for(&server.uri()));
        client.logout().await.unwrap();
    }
}
