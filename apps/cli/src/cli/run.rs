use crate::api::ApiClient;
use anyhow::{bail, Context, Result};
use std::env;
use std::process::Command;

pub async fn execute(
    client: &ApiClient,
    vault: &str,
    env_name: &str,
    command: &[String],
) -> Result<()> {
    if command.is_empty() {
        bail!("No command provided. Usage: secrets run --vault <vault> --env <env> -- <command>");
    }

    // Resolve vault name to ID
    let vault_id = if let Some(v) = client.find_vault_by_name(vault).await? {
        v.id
    } else {
        vault.to_string()
    };

    // Resolve environment name to ID
    let env_obj = client
        .find_environment_by_name(&vault_id, env_name)
        .await?
        .context(format!(
            "Environment '{}' not found in vault '{}'",
            env_name, vault
        ))?;

    // Get all secrets
    let secrets = client.list_secrets(&env_obj.id).await?;

    // Reveal all secret values and build environment map
    let mut secret_env_vars = Vec::new();
    for secret in &secrets {
        let value = client
            .reveal_secret(&secret.id)
            .await
            .context(format!("Failed to reveal secret '{}'", secret.key_name))?;
        secret_env_vars.push((secret.key_name.clone(), value));
    }

    if secret_env_vars.is_empty() {
        eprintln!(
            "Warning: No secrets found in environment '{}/{}'.",
            vault, env_name
        );
    } else {
        eprintln!(
            "Injecting {} secret(s) into environment...",
            secret_env_vars.len()
        );
    }

    // Build command
    let program = &command[0];
    let args = &command[1..];

    // Execute command with secrets injected as environment variables
    let mut cmd = Command::new(program);
    cmd.args(args);

    // Inherit current environment variables
    for (key, value) in env::vars() {
        cmd.env(key, value);
    }

    // Add secret environment variables (will override any existing ones)
    for (key, value) in secret_env_vars {
        cmd.env(key, value);
    }

    // Execute and wait for completion
    let status = cmd
        .status()
        .context(format!("Failed to execute command: {}", program))?;

    // Exit with the same code as the child process
    std::process::exit(status.code().unwrap_or(1));
}
