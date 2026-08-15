use crate::api::ApiClient;
use crate::utils::TablePrinter;
use anyhow::{bail, Result};
use chrono::{DateTime, Duration, NaiveDate, SecondsFormat, TimeZone, Utc};

pub async fn execute(vault: Option<&str>, since: Option<&str>) -> Result<()> {
    let start_date = since
        .map(|s| parse_since(s, Utc::now()))
        .transpose()?
        .map(|t| t.to_rfc3339_opts(SecondsFormat::Secs, true));

    let client = ApiClient::new();

    // Resolve vault name to ID if provided
    let vault_id = if let Some(v) = vault {
        Some(
            if let Some(vault_obj) = client.find_vault_by_name(v).await? {
                vault_obj.id
            } else {
                v.to_string()
            },
        )
    } else {
        None
    };

    let page = client
        .get_audit_logs(vault_id.as_deref(), start_date.as_deref())
        .await?;
    let logs = page.data;

    if logs.is_empty() {
        println!("No audit logs found.");
        return Ok(());
    }

    TablePrinter::print_audit_logs(&logs);
    println!("\nShowing {} of {} events", logs.len(), page.total);

    Ok(())
}

/// Parses `--since` as a relative duration (`30m`, `12h`, `7d`, `2w`), a date (`2026-09-01`,
/// midnight UTC) or an RFC 3339 timestamp, and returns the start of the window.
pub fn parse_since(value: &str, now: DateTime<Utc>) -> Result<DateTime<Utc>> {
    let value = value.trim();

    if let Ok(t) = DateTime::parse_from_rfc3339(value) {
        return Ok(t.with_timezone(&Utc));
    }
    if let Ok(day) = NaiveDate::parse_from_str(value, "%Y-%m-%d") {
        return Ok(Utc.from_utc_datetime(&day.and_hms_opt(0, 0, 0).expect("midnight is valid")));
    }

    let (amount, unit) = value.split_at(value.len().saturating_sub(1));
    let amount: i64 = match amount.parse() {
        Ok(n) if n > 0 => n,
        _ => bail!(
            "invalid --since value '{}': use a duration like 30m, 12h, 7d or 2w, a date (YYYY-MM-DD) or an RFC 3339 timestamp",
            value
        ),
    };
    let duration = match unit {
        "m" => Duration::minutes(amount),
        "h" => Duration::hours(amount),
        "d" => Duration::days(amount),
        "w" => Duration::weeks(amount),
        _ => bail!(
            "invalid --since unit in '{}': use m (minutes), h (hours), d (days) or w (weeks)",
            value
        ),
    };
    Ok(now - duration)
}

#[cfg(test)]
mod tests {
    use super::*;

    fn now() -> DateTime<Utc> {
        Utc.with_ymd_and_hms(2026, 9, 14, 12, 0, 0).unwrap()
    }

    #[test]
    fn parses_relative_durations() {
        assert_eq!(
            parse_since("30m", now()).unwrap(),
            Utc.with_ymd_and_hms(2026, 9, 14, 11, 30, 0).unwrap()
        );
        assert_eq!(
            parse_since("24h", now()).unwrap(),
            Utc.with_ymd_and_hms(2026, 9, 13, 12, 0, 0).unwrap()
        );
        assert_eq!(
            parse_since("7d", now()).unwrap(),
            Utc.with_ymd_and_hms(2026, 9, 7, 12, 0, 0).unwrap()
        );
        assert_eq!(
            parse_since("2w", now()).unwrap(),
            Utc.with_ymd_and_hms(2026, 8, 31, 12, 0, 0).unwrap()
        );
    }

    #[test]
    fn parses_dates_and_timestamps() {
        assert_eq!(
            parse_since("2026-09-01", now()).unwrap(),
            Utc.with_ymd_and_hms(2026, 9, 1, 0, 0, 0).unwrap()
        );
        assert_eq!(
            parse_since("2026-09-01T08:30:00-06:00", now()).unwrap(),
            Utc.with_ymd_and_hms(2026, 9, 1, 14, 30, 0).unwrap()
        );
    }

    #[test]
    fn rejects_invalid_values() {
        for bad in ["", "7", "d", "0d", "-1h", "7y", "yesterday"] {
            assert!(parse_since(bad, now()).is_err(), "{bad} should be rejected");
        }
    }
}
