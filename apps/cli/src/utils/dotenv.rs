//! Writing `.env` files.
//!
//! Values are written unquoted when they only contain safe characters, in single quotes (taken
//! literally by shells and dotenv loaders) when they contain spaces or other symbols, and in
//! double quotes with `\n`, `\r`, `\t`, `\"` and `\\` escapes when they contain a single quote or
//! a line break.

/// Formats one `KEY=value` line (without the trailing newline).
pub fn format_line(key: &str, value: &str) -> String {
    let safe = value
        .chars()
        .all(|c| c.is_ascii_alphanumeric() || "_-./:@%+,=".contains(c));
    if safe {
        return format!("{key}={value}");
    }
    if !value.contains('\'') && !value.contains('\n') && !value.contains('\r') {
        return format!("{key}='{value}'");
    }
    let mut escaped = String::with_capacity(value.len() + 2);
    for c in value.chars() {
        match c {
            '\\' => escaped.push_str("\\\\"),
            '"' => escaped.push_str("\\\""),
            '\n' => escaped.push_str("\\n"),
            '\r' => escaped.push_str("\\r"),
            '\t' => escaped.push_str("\\t"),
            other => escaped.push(other),
        }
    }
    format!("{key}=\"{escaped}\"")
}

/// Formats pairs as a `.env` document, one line each.
pub fn format(pairs: &[(String, String)]) -> String {
    pairs
        .iter()
        .map(|(k, v)| format_line(k, v) + "\n")
        .collect()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn formats_a_document_line_per_pair() {
        let pairs = vec![
            ("A".to_string(), "1".to_string()),
            ("B".to_string(), "two words".to_string()),
        ];
        assert_eq!(format(&pairs), "A=1\nB='two words'\n");
    }

    #[test]
    fn quotes_only_when_needed() {
        assert_eq!(
            format_line("URL", "postgres://u@db:5432/app"),
            "URL=postgres://u@db:5432/app"
        );
        assert_eq!(format_line("EMPTY", ""), "EMPTY=");
        assert_eq!(
            format_line("GREETING", "hello world #1"),
            "GREETING='hello world #1'"
        );
        assert_eq!(format_line("QUOTE", "it's"), "QUOTE=\"it's\"");
        assert_eq!(format_line("PEM", "a\nb\\c\"d"), "PEM=\"a\\nb\\\\c\\\"d\"");
    }
}
