use anyhow::Result;
use api::ApiClient;
use clap::{CommandFactory, Parser, Subcommand};
use config::Settings;
use utils::OutputFormat;

mod api;
mod cli;
mod config;
mod utils;

#[derive(Parser)]
#[command(name = "secrets", version)]
#[command(about = "DevOps Secrets Manager CLI", long_about = None)]
struct Cli {
    /// API base URL [env: SECRETS_API_URL] (default: the URL saved at login, or http://localhost:8080)
    #[arg(long, global = true, value_name = "URL")]
    api_url: Option<String>,

    /// Output format for list commands
    #[arg(long, short = 'o', global = true, value_enum, default_value_t = OutputFormat::Table)]
    output: OutputFormat,

    #[command(subcommand)]
    command: Commands,
}

#[derive(Subcommand)]
enum Commands {
    /// Login to the secrets manager
    Login {
        /// Email address (optional, will prompt if not provided)
        #[arg(long)]
        email: Option<String>,

        /// Password (optional, will prompt if not provided)
        #[arg(long)]
        password: Option<String>,
    },

    /// Logout and clear stored credentials
    Logout,

    /// Vault operations
    Vault {
        #[command(subcommand)]
        command: VaultCommands,
    },

    /// Environment operations
    Env {
        #[command(subcommand)]
        command: EnvCommands,
    },

    /// Vault membership
    Members {
        #[command(subcommand)]
        command: MemberCommands,
    },

    /// Import KEY=value pairs from a .env file into an environment
    Import {
        /// Path to the .env file, or - for standard input
        file: String,

        /// Vault name or ID
        #[arg(long)]
        vault: String,

        /// Environment name
        #[arg(long)]
        env: String,

        /// Update keys that already exist (by default they are skipped)
        #[arg(long)]
        overwrite: bool,

        /// Show what would change without changing anything
        #[arg(long)]
        dry_run: bool,
    },

    /// List secret names in an environment (values are not shown)
    List {
        /// Vault name or ID
        #[arg(long)]
        vault: String,

        /// Environment name
        #[arg(long)]
        env: String,
    },

    /// Pull secrets from an environment
    ///
    /// With SECRETS_TOKEN set, the service token's environment is used and --vault/--env are optional.
    Pull {
        /// Vault name or ID
        #[arg(long)]
        vault: Option<String>,

        /// Environment name
        #[arg(long)]
        env: Option<String>,

        /// Output file (default: stdout)
        #[arg(long)]
        out: Option<String>,
    },

    /// Run a command with secrets injected as environment variables
    ///
    /// With SECRETS_TOKEN set, the service token's environment is used and --vault/--env are optional.
    Run {
        /// Vault name or ID
        #[arg(long)]
        vault: Option<String>,

        /// Environment name
        #[arg(long)]
        env: Option<String>,

        /// Replace secret values that appear in the command's output with [MASKED]
        #[arg(long)]
        mask: bool,

        /// Command to run with arguments
        #[arg(trailing_var_arg = true, allow_hyphen_values = true)]
        command: Vec<String>,
    },

    /// Create a secret, or update its value if the key already exists
    Set {
        /// Secret in KEY=value format
        #[arg(value_name = "KEY=VALUE")]
        secret: String,

        /// Vault name or ID
        #[arg(long)]
        vault: String,

        /// Environment name
        #[arg(long)]
        env: String,

        /// Description of the secret
        #[arg(long)]
        description: Option<String>,

        /// Rotation interval in days
        #[arg(long)]
        rotation_days: Option<i32>,

        /// Fail instead of updating when the key already exists
        #[arg(long, conflicts_with = "update_only")]
        create_only: bool,

        /// Fail instead of creating when the key does not exist
        #[arg(long)]
        update_only: bool,
    },

    /// Delete a secret from an environment
    Delete {
        /// Key name of the secret
        #[arg(value_name = "KEY")]
        key: String,

        /// Vault name or ID
        #[arg(long)]
        vault: String,

        /// Environment name
        #[arg(long)]
        env: String,

        /// Skip the confirmation prompt (required when not running in a terminal)
        #[arg(long, short = 'y')]
        yes: bool,
    },

    /// Print a shell completion script (e.g. `secrets completions fish > ~/.config/fish/completions/secrets.fish`)
    Completions {
        /// Shell to generate completions for
        shell: clap_complete::Shell,
    },

    /// View audit logs
    Audit {
        /// Filter by vault name
        #[arg(long)]
        vault: Option<String>,

        /// Only show events since a duration ago (30m, 24h, 7d, 2w), a date (YYYY-MM-DD) or an RFC 3339 time
        #[arg(long)]
        since: Option<String>,
    },
}

#[derive(Subcommand)]
enum VaultCommands {
    /// List all vaults
    List,

    /// Create a vault (you become its owner)
    Create {
        /// Vault name
        name: String,

        /// Description
        #[arg(long)]
        description: Option<String>,

        /// Organization ID (default: your first organization)
        #[arg(long, value_name = "ID")]
        org: Option<String>,
    },
}

#[derive(Subcommand)]
enum MemberCommands {
    /// List a vault's members and their roles
    List {
        /// Vault name or ID
        #[arg(long)]
        vault: String,
    },

    /// Add someone from the vault's organization to the vault
    Add {
        /// Their account email
        email: String,

        /// Role on the vault
        #[arg(long, value_parser = clap::builder::PossibleValuesParser::new(cli::members::ROLES))]
        role: String,

        /// Vault name or ID
        #[arg(long)]
        vault: String,
    },

    /// Remove someone from the vault
    Remove {
        /// Their account email
        email: String,

        /// Vault name or ID
        #[arg(long)]
        vault: String,
    },
}

#[derive(Subcommand)]
enum EnvCommands {
    /// List environments in a vault
    List {
        /// Vault name or ID
        #[arg(long)]
        vault: String,
    },

    /// Create an environment in a vault
    Create {
        /// Environment name (lowercase letters, digits, dots, hyphens, underscores)
        name: String,

        /// Vault name or ID
        #[arg(long)]
        vault: String,

        /// Description
        #[arg(long)]
        description: Option<String>,
    },
}

#[tokio::main]
async fn main() -> Result<()> {
    let cli = Cli::parse();

    // Completions need no configuration or network access
    if let Commands::Completions { shell } = cli.command {
        let mut script = Vec::new();
        clap_complete::generate(shell, &mut Cli::command(), "secrets", &mut script);
        use std::io::Write;
        return match std::io::stdout().write_all(&script) {
            // A closed pipe (e.g. `| head`) is not an error worth reporting
            Err(e) if e.kind() == std::io::ErrorKind::BrokenPipe => Ok(()),
            other => Ok(other?),
        };
    }

    let settings = Settings::load();
    let env_url = std::env::var(config::settings::API_URL_ENV).ok();
    let api_url =
        config::settings::resolve_api_url(cli.api_url.as_deref(), env_url.as_deref(), &settings)?;
    let client = ApiClient::new(api_url);

    // A service token only grants run and pull; other commands need a logged-in session
    let token = std::env::var(cli::source::TOKEN_ENV)
        .ok()
        .filter(|t| !t.trim().is_empty());
    if token.is_some() && !matches!(cli.command, Commands::Run { .. } | Commands::Pull { .. }) {
        anyhow::bail!(
            "{} is set, but service tokens only work with 'secrets run' and 'secrets pull'; unset it to use your login",
            cli::source::TOKEN_ENV
        );
    }

    match cli.command {
        Commands::Login { email, password } => {
            // An explicit --api-url is remembered for later commands
            let save_url = cli.api_url.is_some();
            cli::login::execute(&client, email, password, save_url).await
        }
        Commands::Logout => cli::logout::execute(&client).await,
        Commands::Vault { command } => match command {
            VaultCommands::List => cli::vault::list(&client, cli.output).await,
            VaultCommands::Create {
                name,
                description,
                org,
            } => cli::vault::create(&client, &name, description.as_deref(), org.as_deref()).await,
        },
        Commands::Env { command } => match command {
            EnvCommands::List { vault } => cli::env::list(&client, &vault, cli.output).await,
            EnvCommands::Create {
                name,
                vault,
                description,
            } => cli::env::create(&client, &vault, &name, description.as_deref()).await,
        },
        Commands::Members { command } => match command {
            MemberCommands::List { vault } => cli::members::list(&client, &vault, cli.output).await,
            MemberCommands::Add { email, role, vault } => {
                cli::members::add(&client, &vault, &email, &role).await
            }
            MemberCommands::Remove { email, vault } => {
                cli::members::remove(&client, &vault, &email).await
            }
        },
        Commands::Import {
            file,
            vault,
            env,
            overwrite,
            dry_run,
        } => cli::import::execute(&client, &file, &vault, &env, overwrite, dry_run).await,
        Commands::List { vault, env } => {
            cli::list::execute(&client, &vault, &env, cli.output).await
        }
        Commands::Pull { vault, env, out } => {
            cli::pull::execute(
                &client,
                token.as_deref(),
                vault.as_deref(),
                env.as_deref(),
                out.as_deref(),
            )
            .await
        }
        Commands::Run {
            vault,
            env,
            mask,
            command,
        } => {
            cli::run::execute(
                &client,
                token.as_deref(),
                vault.as_deref(),
                env.as_deref(),
                &command,
                mask,
            )
            .await
        }
        Commands::Set {
            secret,
            vault,
            env,
            description,
            rotation_days,
            create_only,
            update_only,
        } => {
            cli::set::execute(
                &client,
                &secret,
                &vault,
                &env,
                description.as_deref(),
                rotation_days,
                cli::set::SetMode::from_flags(create_only, update_only),
            )
            .await
        }
        Commands::Delete {
            key,
            vault,
            env,
            yes,
        } => cli::delete::execute(&client, &key, &vault, &env, yes).await,
        Commands::Completions { .. } => unreachable!("handled before configuration is loaded"),
        Commands::Audit { vault, since } => {
            cli::audit::execute(&client, vault.as_deref(), since.as_deref(), cli.output).await
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn command_definition_is_valid() {
        Cli::command().debug_assert();
    }

    #[test]
    fn parses_run_with_trailing_command_and_flags() {
        let cli = Cli::try_parse_from([
            "secrets",
            "--api-url",
            "https://api.example",
            "run",
            "--vault",
            "app",
            "--env",
            "production",
            "--mask",
            "--",
            "npm",
            "run",
            "deploy",
            "--prod",
        ])
        .unwrap();
        assert_eq!(cli.api_url.as_deref(), Some("https://api.example"));
        match cli.command {
            Commands::Run {
                vault,
                env,
                mask,
                command,
            } => {
                assert_eq!(
                    (vault.as_deref(), env.as_deref(), mask),
                    (Some("app"), Some("production"), true)
                );
                assert_eq!(command, ["npm", "run", "deploy", "--prod"]);
            }
            _ => panic!("expected run"),
        }
    }

    #[test]
    fn global_flags_work_after_the_subcommand() {
        let cli = Cli::try_parse_from(["secrets", "vault", "list", "-o", "json"]).unwrap();
        assert_eq!(cli.output, OutputFormat::Json);
    }

    #[test]
    fn rejects_conflicting_and_missing_arguments() {
        assert!(Cli::try_parse_from([
            "secrets",
            "set",
            "A=1",
            "--vault",
            "v",
            "--env",
            "e",
            "--create-only",
            "--update-only"
        ])
        .is_err());
        assert!(Cli::try_parse_from(["secrets", "delete", "KEY", "--vault", "v"]).is_err());
        assert!(Cli::try_parse_from([
            "secrets", "members", "add", "a@b.c", "--vault", "v", "--role", "root"
        ])
        .is_err());
    }

    #[test]
    fn completions_are_generated_for_each_shell() {
        for shell in [
            clap_complete::Shell::Bash,
            clap_complete::Shell::Fish,
            clap_complete::Shell::Zsh,
        ] {
            let mut buf = Vec::new();
            clap_complete::generate(shell, &mut Cli::command(), "secrets", &mut buf);
            let script = String::from_utf8(buf).unwrap();
            assert!(
                script.contains("import"),
                "{shell:?} completions should mention subcommands"
            );
        }
    }
}
