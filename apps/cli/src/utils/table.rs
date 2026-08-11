use comfy_table::{Attribute, Cell, Color, ContentArrangement, Table};

pub struct TablePrinter;

impl TablePrinter {
    pub fn print_vaults(vaults: &[crate::api::client::Vault]) {
        let mut table = Table::new();
        table.set_content_arrangement(ContentArrangement::Dynamic);

        table.set_header(vec![
            Cell::new("ID")
                .fg(Color::Green)
                .add_attribute(Attribute::Bold),
            Cell::new("Name")
                .fg(Color::Green)
                .add_attribute(Attribute::Bold),
            Cell::new("Description")
                .fg(Color::Green)
                .add_attribute(Attribute::Bold),
            Cell::new("Created At")
                .fg(Color::Green)
                .add_attribute(Attribute::Bold),
        ]);

        for vault in vaults {
            table.add_row(vec![
                &vault.id,
                &vault.name,
                vault.description.as_deref().unwrap_or("-"),
                &vault.created_at,
            ]);
        }

        println!("{}", table);
    }

    pub fn print_environments(environments: &[crate::api::client::Environment]) {
        let mut table = Table::new();
        table.set_content_arrangement(ContentArrangement::Dynamic);

        table.set_header(vec![
            Cell::new("ID")
                .fg(Color::Green)
                .add_attribute(Attribute::Bold),
            Cell::new("Name")
                .fg(Color::Green)
                .add_attribute(Attribute::Bold),
            Cell::new("Description")
                .fg(Color::Green)
                .add_attribute(Attribute::Bold),
            Cell::new("Vault ID")
                .fg(Color::Green)
                .add_attribute(Attribute::Bold),
        ]);

        for env in environments {
            table.add_row(vec![
                &env.id,
                &env.name,
                env.description.as_deref().unwrap_or("-"),
                &env.vault_id,
            ]);
        }

        println!("{}", table);
    }

    pub fn print_secrets(secrets: &[crate::api::client::Secret]) {
        let mut table = Table::new();
        table.set_content_arrangement(ContentArrangement::Dynamic);

        table.set_header(vec![
            Cell::new("ID")
                .fg(Color::Green)
                .add_attribute(Attribute::Bold),
            Cell::new("Key Name")
                .fg(Color::Green)
                .add_attribute(Attribute::Bold),
            Cell::new("Description")
                .fg(Color::Green)
                .add_attribute(Attribute::Bold),
            Cell::new("Rotation Days")
                .fg(Color::Green)
                .add_attribute(Attribute::Bold),
            Cell::new("Expires At")
                .fg(Color::Green)
                .add_attribute(Attribute::Bold),
        ]);

        for secret in secrets {
            table.add_row(vec![
                &secret.id,
                &secret.key_name,
                secret.description.as_deref().unwrap_or("-"),
                &secret
                    .rotation_interval_days
                    .map(|d| d.to_string())
                    .unwrap_or_else(|| "-".to_string()),
                secret.expires_at.as_deref().unwrap_or("-"),
            ]);
        }

        println!("{}", table);
    }

    pub fn print_audit_logs(logs: &[crate::api::client::AuditLog]) {
        let mut table = Table::new();
        table.set_content_arrangement(ContentArrangement::Dynamic);

        table.set_header(vec![
            Cell::new("Timestamp")
                .fg(Color::Green)
                .add_attribute(Attribute::Bold),
            Cell::new("Action")
                .fg(Color::Green)
                .add_attribute(Attribute::Bold),
            Cell::new("User ID")
                .fg(Color::Green)
                .add_attribute(Attribute::Bold),
            Cell::new("Vault ID")
                .fg(Color::Green)
                .add_attribute(Attribute::Bold),
            Cell::new("IP Address")
                .fg(Color::Green)
                .add_attribute(Attribute::Bold),
        ]);

        for log in logs {
            table.add_row(vec![
                &log.timestamp,
                &log.action,
                &log.user_id,
                log.vault_id.as_deref().unwrap_or("-"),
                log.ip_address.as_deref().unwrap_or("-"),
            ]);
        }

        println!("{}", table);
    }
}
