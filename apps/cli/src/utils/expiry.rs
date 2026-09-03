use chrono::{DateTime, Utc};

/// Secrets expiring within this many days get a notice.
pub const WARNING_DAYS: i64 = 14;

/// Describes a secret's expiry for display: "expired", "expires in 3d", or None when there is
/// no expiry, it cannot be parsed, or it is far enough away.
pub fn describe(expires_at: Option<&str>, now: DateTime<Utc>) -> Option<String> {
    let expires = DateTime::parse_from_rfc3339(expires_at?)
        .ok()?
        .with_timezone(&Utc);
    let remaining = expires - now;
    if remaining.num_seconds() <= 0 {
        return Some(format!("expired {}", expires.format("%Y-%m-%d")));
    }
    let days = remaining.num_days();
    (days < WARNING_DAYS).then(|| format!("expires in {days}d ({})", expires.format("%Y-%m-%d")))
}

/// Describes a rotation due date: "overdue since 2026-09-01", "due in 3d", or None when it is
/// more than a week away or cannot be parsed.
pub fn describe_rotation(next_rotation_at: &str, now: DateTime<Utc>) -> Option<String> {
    let due = DateTime::parse_from_rfc3339(next_rotation_at)
        .ok()?
        .with_timezone(&Utc);
    let remaining = due - now;
    if remaining.num_seconds() <= 0 {
        return Some(format!("overdue since {}", due.format("%Y-%m-%d")));
    }
    let days = remaining.num_days();
    (days < 7).then(|| format!("due in {days}d"))
}

/// Warning lines for expired or soon-expiring secrets, printed to stderr by run and pull.
/// Expired values are still used: a stale credential is the caller's call, but it should be loud.
pub fn warnings<'a>(
    items: impl IntoIterator<Item = (&'a str, Option<&'a str>)>,
    now: DateTime<Utc>,
) -> Vec<String> {
    items
        .into_iter()
        .filter_map(|(key, expires_at)| {
            describe(expires_at, now).map(|d| {
                let level = if d.starts_with("expired") {
                    "Warning"
                } else {
                    "Note"
                };
                format!("{level}: secret {key} {d}; rotate it")
            })
        })
        .collect()
}

#[cfg(test)]
mod tests {
    use super::*;

    fn now() -> DateTime<Utc> {
        DateTime::parse_from_rfc3339("2026-09-15T12:00:00Z")
            .unwrap()
            .with_timezone(&Utc)
    }

    #[test]
    fn describes_expired_soon_and_distant_dates() {
        assert_eq!(
            describe(Some("2026-09-10T00:00:00Z"), now()).as_deref(),
            Some("expired 2026-09-10")
        );
        assert_eq!(
            describe(Some("2026-09-18T12:00:00Z"), now()).as_deref(),
            Some("expires in 3d (2026-09-18)")
        );
        assert_eq!(describe(Some("2026-12-01T00:00:00Z"), now()), None);
        assert_eq!(describe(None, now()), None);
        assert_eq!(describe(Some("not a date"), now()), None);
    }

    #[test]
    fn describes_rotation_due_dates() {
        assert_eq!(
            describe_rotation("2026-09-01T00:00:00Z", now()).as_deref(),
            Some("overdue since 2026-09-01")
        );
        assert_eq!(
            describe_rotation("2026-09-17T13:00:00Z", now()).as_deref(),
            Some("due in 2d")
        );
        assert_eq!(describe_rotation("2026-10-30T00:00:00Z", now()), None);
    }

    #[test]
    fn warns_only_about_expired_or_expiring_secrets() {
        let lines = warnings(
            [
                ("OLD", Some("2026-09-01T00:00:00Z")),
                ("SOON", Some("2026-09-20T12:00:00Z")),
                ("FINE", None),
            ],
            now(),
        );
        assert_eq!(
            lines,
            vec![
                "Warning: secret OLD expired 2026-09-01; rotate it".to_string(),
                "Note: secret SOON expires in 5d (2026-09-20); rotate it".to_string(),
            ]
        );
    }
}
