//! Reading and writing `.env` files.
//!
//! Values are written unquoted when they only contain safe characters, in single quotes (taken
//! literally by shells and dotenv loaders) when they contain spaces or other symbols, and in
//! double quotes with `\n`, `\r`, `\t`, `\"` and `\\` escapes when they contain a single quote or
//! a line break.

use std::fmt;

/// A problem in a `.env` file, with the 1-based line number where it starts.
#[derive(Debug, PartialEq)]
pub struct ParseError {
    pub line: usize,
    pub message: String,
}

impl fmt::Display for ParseError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "line {}: {}", self.line, self.message)
    }
}

impl std::error::Error for ParseError {}

/// Whether `key` is a valid environment variable name.
pub fn is_valid_key(key: &str) -> bool {
    let mut chars = key.chars();
    matches!(chars.next(), Some(c) if c.is_ascii_alphabetic() || c == '_')
        && chars.all(|c| c.is_ascii_alphanumeric() || c == '_')
}

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

/// Parses a `.env` document. Blank lines and `#` comments are skipped, an `export ` prefix is
/// allowed, later duplicates override earlier ones (and keep the first position), and double
/// quoted values may span lines.
pub fn parse(content: &str) -> Result<Vec<(String, String)>, ParseError> {
    let mut pairs: Vec<(String, String)> = Vec::new();
    let lines: Vec<&str> = content.lines().collect();
    let mut i = 0;

    while i < lines.len() {
        let line_no = i + 1;
        let line = lines[i].trim();
        i += 1;
        if line.is_empty() || line.starts_with('#') {
            continue;
        }
        let line = line
            .strip_prefix("export ")
            .map(str::trim_start)
            .unwrap_or(line);

        let (key, rest) = line.split_once('=').ok_or_else(|| ParseError {
            line: line_no,
            message: "expected KEY=value".into(),
        })?;
        let key = key.trim();
        if !is_valid_key(key) {
            return Err(ParseError {
                line: line_no,
                message: format!("'{key}' is not a valid variable name"),
            });
        }
        let rest = rest.trim_start();

        let value = if let Some(body) = rest.strip_prefix('\'') {
            let end = body.find('\'').ok_or_else(|| ParseError {
                line: line_no,
                message: "unterminated single-quoted value".into(),
            })?;
            body[..end].to_string()
        } else if let Some(body) = rest.strip_prefix('"') {
            // Collect until an unescaped closing quote, possibly across lines
            let mut raw = body.to_string();
            loop {
                if let Some(end) = closing_quote(&raw) {
                    raw.truncate(end);
                    break;
                }
                if i >= lines.len() {
                    return Err(ParseError {
                        line: line_no,
                        message: "unterminated double-quoted value".into(),
                    });
                }
                raw.push('\n');
                raw.push_str(lines[i]);
                i += 1;
            }
            unescape(&raw)
        } else {
            // Unquoted: an inline comment starts at " #"
            let value = match rest.find(" #") {
                Some(pos) => &rest[..pos],
                None => rest,
            };
            value.trim_end().to_string()
        };

        match pairs.iter_mut().find(|(k, _)| k == key) {
            Some(existing) => existing.1 = value,
            None => pairs.push((key.to_string(), value)),
        }
    }

    Ok(pairs)
}

fn closing_quote(s: &str) -> Option<usize> {
    let mut escaped = false;
    for (idx, c) in s.char_indices() {
        match c {
            '\\' if !escaped => escaped = true,
            '"' if !escaped => return Some(idx),
            _ => escaped = false,
        }
    }
    None
}

fn unescape(s: &str) -> String {
    let mut out = String::with_capacity(s.len());
    let mut chars = s.chars();
    while let Some(c) = chars.next() {
        if c != '\\' {
            out.push(c);
            continue;
        }
        match chars.next() {
            Some('n') => out.push('\n'),
            Some('r') => out.push('\r'),
            Some('t') => out.push('\t'),
            Some('"') => out.push('"'),
            Some('\\') => out.push('\\'),
            Some(other) => {
                out.push('\\');
                out.push(other);
            }
            None => out.push('\\'),
        }
    }
    out
}

#[cfg(test)]
mod tests {
    use super::*;

    fn pairs(items: &[(&str, &str)]) -> Vec<(String, String)> {
        items
            .iter()
            .map(|(k, v)| (k.to_string(), v.to_string()))
            .collect()
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

    #[test]
    fn round_trips_awkward_values() {
        let original = pairs(&[
            ("PLAIN", "abc123"),
            ("SPACES", "  leading and trailing  "),
            ("HASH", "pa#ss word"),
            ("DOLLAR", "$HOME and ${PATH}"),
            ("QUOTES", "it's \"quoted\""),
            ("MULTILINE", "-----BEGIN KEY-----\nabc\n-----END KEY-----"),
            ("BACKSLASH", "C:\\path\\to"),
            ("EQUALS", "a=b=c"),
            ("UNICODE", "pässwörd ✓"),
            ("EMPTY", ""),
        ]);
        assert_eq!(parse(&format(&original)).unwrap(), original);
    }

    #[test]
    fn parses_common_dotenv_syntax() {
        let content = "# comment\n\nexport API_KEY=sk_test_123\nDB_URL = postgres://x # inline comment\nSINGLE='literal \\n $value'\nDOUBLE=\"line1\\nline2\"\nMULTI=\"first\nsecond\"\nAPI_KEY=override\n";
        assert_eq!(
            parse(content).unwrap(),
            pairs(&[
                ("API_KEY", "override"),
                ("DB_URL", "postgres://x"),
                ("SINGLE", "literal \\n $value"),
                ("DOUBLE", "line1\nline2"),
                ("MULTI", "first\nsecond"),
            ])
        );
    }

    #[test]
    fn reports_line_numbers_for_errors() {
        assert_eq!(parse("A=1\nnot a pair\n").unwrap_err().line, 2);
        assert_eq!(parse("\n1BAD=x").unwrap_err().line, 2);
        assert!(parse("A=\"never closed\nB=2")
            .unwrap_err()
            .message
            .contains("unterminated"));
        assert!(parse("A='open").is_err());
    }

    #[test]
    fn validates_keys() {
        assert!(is_valid_key("_PRIVATE_2"));
        assert!(!is_valid_key("2FAST"));
        assert!(!is_valid_key("WITH-DASH"));
        assert!(!is_valid_key(""));
    }
}
