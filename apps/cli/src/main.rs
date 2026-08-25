use anyhow::Result;
use api::ApiClient;
use clap::{Parser, Subcommand};
use config::Settings;

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

    /// Pull secrets from an environment
    Pull {
        /// Vault name or ID
        #[arg(long)]
        vault: String,

        /// Environment name
        #[arg(long)]
        env: String,

        /// Output file (default: stdout)
        #[arg(long)]
        out: Option<String>,
    },

    /// Run a command with secrets injected as environment variables
    Run {
        /// Vault name or ID
        #[arg(long)]
        vault: String,

        /// Environment name
        #[arg(long)]
        env: String,

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
}

#[derive(Subcommand)]
enum EnvCommands {
    /// List environments in a vault
    List {
        /// Vault name or ID
        #[arg(long)]
        vault: String,
    },
}

#[tokio::main]
async fn main() -> Result<()> {
    let cli = Cli::parse();

    let settings = Settings::load();
    let env_url = std::env::var(config::settings::API_URL_ENV).ok();
    let api_url =
        config::settings::resolve_api_url(cli.api_url.as_deref(), env_url.as_deref(), &settings)?;
    let client = ApiClient::new(api_url);

    match cli.command {
        Commands::Login { email, password } => {
            // An explicit --api-url is remembered for later commands
            let save_url = cli.api_url.is_some();
            cli::login::execute(&client, email, password, save_url).await
        }
        Commands::Logout => cli::logout::execute(&client).await,
        Commands::Vault { command } => match command {
            VaultCommands::List => cli::vault::list(&client).await,
        },
        Commands::Env { command } => match command {
            EnvCommands::List { vault } => cli::env::list(&client, &vault).await,
        },
        Commands::Pull { vault, env, out } => {
            cli::pull::execute(&client, &vault, &env, out.as_deref()).await
        }
        Commands::Run {
            vault,
            env,
            command,
        } => cli::run::execute(&client, &vault, &env, &command).await,
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
        Commands::Audit { vault, since } => {
            cli::audit::execute(&client, vault.as_deref(), since.as_deref()).await
        }
    }
}
