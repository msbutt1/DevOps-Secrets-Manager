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

    pub fn print_audit_logs(logs: &[crate::api::client::AuditLog]) {
        let mut table = Table::new();
        table.set_content_arrangement(ContentArrangement::Dynamic);

        let header = [
            "Timestamp",
            "Action",
            "User",
            "Vault",
            "Environment",
            "Target",
            "IP Address",
        ];
        table.set_header(
            header
                .iter()
                .map(|h| Cell::new(h).fg(Color::Green).add_attribute(Attribute::Bold)),
        );

        for log in logs {
            table.add_row(vec![
                log.timestamp.as_str(),
                log.action.as_str(),
                log.user_email.as_str(),
                log.vault_name.as_deref().unwrap_or("-"),
                log.environment_name.as_deref().unwrap_or("-"),
                log.target_name.as_deref().unwrap_or("-"),
                log.ip_address.as_deref().unwrap_or("-"),
            ]);
        }

        println!("{}", table);
    }

    pub fn print_members(members: &[crate::api::client::Member]) {
        let mut table = Table::new();
        table.set_content_arrangement(ContentArrangement::Dynamic);
        table.set_header(
            ["Email", "Name", "Role", "Added By", "Added At"]
                .iter()
                .map(|h| Cell::new(h).fg(Color::Green).add_attribute(Attribute::Bold)),
        );
        for m in members {
            table.add_row(vec![
                m.email.as_str(),
                m.name.as_str(),
                m.role.as_str(),
                if m.added_by.is_empty() {
                    "-"
                } else {
                    m.added_by.as_str()
                },
                m.added_at.as_str(),
            ]);
        }
        println!("{}", table);
    }

    pub fn print_secrets(secrets: &[crate::api::client::Secret]) {
        let mut table = Table::new();
        table.set_content_arrangement(ContentArrangement::Dynamic);
        table.set_header(
            ["Key", "Description", "Rotation", "Expires"]
                .iter()
                .map(|h| Cell::new(h).fg(Color::Green).add_attribute(Attribute::Bold)),
        );
        for s in secrets {
            table.add_row(vec![
                s.key_name.clone(),
                s.description.clone().unwrap_or_else(|| "-".into()),
                s.rotation_interval_days
                    .map(|d| format!("every {d}d"))
                    .unwrap_or_else(|| "-".into()),
                match (
                    &s.expires_at,
                    super::expiry::describe(s.expires_at.as_deref(), chrono::Utc::now()),
                ) {
                    (Some(_), Some(status)) => status,
                    (Some(date), None) => chrono::DateTime::parse_from_rfc3339(date)
                        .map(|d| d.with_timezone(&chrono::Utc).format("%Y-%m-%d").to_string())
                        .unwrap_or_else(|_| date.clone()),
                    (None, _) => "-".into(),
                },
            ]);
        }
        println!("{}", table);
    }
}
