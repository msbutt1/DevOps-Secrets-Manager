use anyhow::{Context, Result};
use std::fs::{self, OpenOptions};
use std::io::Write;
use std::path::Path;

/// Writes `contents` to `path` so that only the current user can read it (mode 0600 on Unix).
/// An existing file is truncated and its permissions tightened before anything is written.
pub fn write_private(path: &Path, contents: &str) -> Result<()> {
    let mut options = OpenOptions::new();
    options.write(true).create(true).truncate(true);
    #[cfg(unix)]
    {
        use std::os::unix::fs::OpenOptionsExt;
        options.mode(0o600);
    }

    let mut file = options
        .open(path)
        .with_context(|| format!("Could not open {} for writing", path.display()))?;

    #[cfg(unix)]
    {
        use std::os::unix::fs::PermissionsExt;
        // `mode` only applies when the file is created; tighten files that already existed
        fs::set_permissions(path, fs::Permissions::from_mode(0o600))?;
    }

    file.write_all(contents.as_bytes())?;
    file.flush()?;
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[cfg(unix)]
    #[test]
    fn creates_and_tightens_files_to_owner_only() {
        use std::os::unix::fs::PermissionsExt;
        let dir = std::env::temp_dir().join(format!("secrets-cli-test-{}", std::process::id()));
        fs::create_dir_all(&dir).unwrap();
        let path = dir.join(".env");

        fs::write(&path, "OLD=1\n").unwrap();
        fs::set_permissions(&path, fs::Permissions::from_mode(0o644)).unwrap();

        write_private(&path, "KEY=value\n").unwrap();
        assert_eq!(fs::read_to_string(&path).unwrap(), "KEY=value\n");
        assert_eq!(
            fs::metadata(&path).unwrap().permissions().mode() & 0o777,
            0o600
        );

        fs::remove_dir_all(&dir).unwrap();
    }
}
