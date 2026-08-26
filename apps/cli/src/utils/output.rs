use anyhow::Result;
use clap::ValueEnum;
use serde::Serialize;

/// How list commands print their results.
#[derive(Debug, Clone, Copy, PartialEq, Eq, ValueEnum, Default)]
pub enum OutputFormat {
    /// Human-readable table
    #[default]
    Table,
    /// JSON array on standard output, for scripts
    Json,
}

/// Prints a value as pretty JSON followed by a newline.
pub fn print_json<T: Serialize + ?Sized>(value: &T) -> Result<()> {
    println!("{}", serde_json::to_string_pretty(value)?);
    Ok(())
}
