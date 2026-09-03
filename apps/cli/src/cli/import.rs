use crate::api::client::{CreateSecretRequest, Secret, UpdateSecretRequest};
use crate::api::ApiClient;
use crate::cli::resolve;
use crate::utils::dotenv;
use anyhow::{bail, Context, Result};
use std::collections::HashMap;
use std::fs;

/// What import does with one key.
#[derive(Debug, PartialEq, Eq)]
pub enum Action {
    Create,
    Update,
    Skip,
}

/// Decides the action for each key: new keys are created; existing keys are updated with
/// --overwrite and skipped otherwise.
pub fn plan<'a>(
    pairs: &'a [(String, String)],
    existing: &HashMap<String, Secret>,
    overwrite: bool,
) -> Vec<(&'a str, &'a str, Action)> {
    pairs
        .iter()
        .map(|(key, value)| {
            let action = match (existing.contains_key(key), overwrite) {
                (false, _) => Action::Create,
                (true, true) => Action::Update,
                (true, false) => Action::Skip,
            };
            (key.as_str(), value.as_str(), action)
        })
        .collect()
}

pub async fn execute(
    client: &ApiClient,
    file: &str,
    vault: &str,
    env_name: &str,
    overwrite: bool,
    dry_run: bool,
) -> Result<()> {
    let content = if file == "-" {
        std::io::read_to_string(std::io::stdin()).context("Could not read standard input")?
    } else {
        fs::read_to_string(file).with_context(|| format!("Could not read {file}"))?
    };
    let pairs = dotenv::parse(&content).with_context(|| format!("Invalid .env file {file}"))?;
    if pairs.is_empty() {
        bail!("{file} contains no KEY=value lines");
    }

    let (vault, env) = resolve::environment(client, vault, env_name).await?;
    let existing: HashMap<String, Secret> = client
        .list_secrets(&env.id)
        .await?
        .into_iter()
        .map(|s| (s.key_name.clone(), s))
        .collect();

    let steps = plan(&pairs, &existing, overwrite);
    let count = |a: Action| steps.iter().filter(|(_, _, x)| *x == a).count();
    let (creates, updates, skips) = (
        count(Action::Create),
        count(Action::Update),
        count(Action::Skip),
    );

    // Preview: key names and actions only, never values
    println!("Import into {}/{}:", vault.name, env.name);
    for (key, _, action) in &steps {
        let label = match action {
            Action::Create => "create",
            Action::Update => "update",
            Action::Skip => "skip (exists; use --overwrite to update)",
        };
        println!("  {key:<40} {label}");
    }
    println!("{creates} to create, {updates} to update, {skips} to skip");

    if dry_run {
        println!("Dry run: nothing was changed.");
        return Ok(());
    }

    for (key, value, action) in steps {
        match action {
            Action::Create => {
                client
                    .create_secret(
                        &env.id,
                        CreateSecretRequest {
                            key_name: key.to_string(),
                            value: value.to_string(),
                            description: None,
                            rotation_interval_days: None,
                            expires_at: None,
                            metadata: None,
                        },
                    )
                    .await
                    .with_context(|| format!("Failed to create {key}"))?;
            }
            Action::Update => {
                let current = &existing[key];
                client
                    .update_secret(
                        &current.id,
                        &UpdateSecretRequest {
                            value: Some(value.to_string()),
                            description: current.description.clone(),
                            rotation_interval_days: current.rotation_interval_days,
                            expires_at: current.expires_at.clone(),
                            metadata: current.metadata.clone(),
                        },
                    )
                    .await
                    .with_context(|| format!("Failed to update {key}"))?;
            }
            Action::Skip => {}
        }
    }
    println!("Imported: {creates} created, {updates} updated, {skips} skipped.");
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    fn secret(key: &str) -> Secret {
        Secret {
            id: format!("id-{key}"),
            key_name: key.into(),
            description: None,
            rotation_interval_days: None,
            expires_at: None,
            metadata: None,
            rotation_policy: None,
        }
    }

    #[test]
    fn plans_create_update_and_skip() {
        let pairs = vec![
            ("NEW".to_string(), "1".to_string()),
            ("OLD".to_string(), "2".to_string()),
        ];
        let existing = HashMap::from([("OLD".to_string(), secret("OLD"))]);

        let actions: Vec<_> = plan(&pairs, &existing, false)
            .into_iter()
            .map(|(k, _, a)| (k, a))
            .collect();
        assert_eq!(
            actions,
            vec![("NEW", Action::Create), ("OLD", Action::Skip)]
        );

        let actions: Vec<_> = plan(&pairs, &existing, true)
            .into_iter()
            .map(|(k, _, a)| (k, a))
            .collect();
        assert_eq!(
            actions,
            vec![("NEW", Action::Create), ("OLD", Action::Update)]
        );
    }
}
