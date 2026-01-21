use clap::{Parser, Subcommand};
use anyhow::Result;

mod api;
mod cli;
mod config;
mod utils;

#[derive(Parser)]
#[command(name = "secrets")]
#[command(about = "DevOps Secrets Manager CLI", long_about = None)]
struct Cli {
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

    /// Set a secret value
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
    },

    /// View audit logs
    Audit {
        /// Filter by vault name
        #[arg(long)]
        vault: Option<String>,

        /// Filter logs since duration (e.g., 1h, 24h, 7d)
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

    match cli.command {
        Commands::Login { email, password } => cli::login::execute(email, password).await,
        Commands::Logout => cli::logout::execute().await,
        Commands::Vault { command } => match command {
            VaultCommands::List => cli::vault::list().await,
        },
        Commands::Env { command } => match command {
            EnvCommands::List { vault } => cli::env::list(&vault).await,
        },
        Commands::Pull { vault, env, out } => {
            cli::pull::execute(&vault, &env, out.as_deref()).await
        },
        Commands::Run { vault, env, command } => {
            cli::run::execute(&vault, &env, &command).await
        },
        Commands::Set { secret, vault, env, description, rotation_days } => {
            cli::set::execute(&secret, &vault, &env, description.as_deref(), rotation_days).await
        },
        Commands::Audit { vault, since } => {
            cli::audit::execute(vault.as_deref(), since.as_deref()).await
        },
    }
}
