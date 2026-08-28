use crate::api::ApiClient;
use crate::cli::source;
use anyhow::{bail, Context, Result};
use std::io::{self, BufRead, BufReader, Read, Write};
use std::process::{Command, ExitStatus, Stdio};
use std::thread;

/// Values shorter than this are not masked: replacing every "1" or "on" in the output would
/// hide useful text without protecting anything.
pub const MIN_MASK_LEN: usize = 4;
const MASK: &[u8] = b"[MASKED]";

pub async fn execute(
    client: &ApiClient,
    token: Option<&str>,
    vault: Option<&str>,
    env_name: Option<&str>,
    command: &[String],
    mask: bool,
) -> Result<()> {
    if command.is_empty() {
        bail!("No command provided. Usage: secrets run [--vault <vault> --env <env>] -- <command>");
    }

    // Values only ever go into the child's environment
    let loaded = source::load(client, token, vault, env_name).await?;
    let secret_env_vars = loaded.pairs;

    if secret_env_vars.is_empty() {
        eprintln!(
            "Warning: No secrets found in environment '{}/{}'.",
            loaded.vault, loaded.environment
        );
    } else {
        eprintln!(
            "Injecting {} secret(s) from {}/{}{}",
            secret_env_vars.len(),
            loaded.vault,
            loaded.environment,
            if mask {
                " (masking values in output)"
            } else {
                ""
            }
        );
    }

    let code = run_child(
        command,
        &secret_env_vars,
        mask,
        &mut io::stdout(),
        &mut io::stderr(),
    )?;

    // Exit with the same code as the child process
    std::process::exit(code);
}

/// Runs the command with the extra environment variables and returns the exit code to use.
/// With `mask`, the child's stdout and stderr are piped and every occurrence of a secret value
/// (at least MIN_MASK_LEN bytes long) is replaced before being written to `out` and `err`.
pub fn run_child<O: Write + Send, E: Write + Send>(
    command: &[String],
    env_vars: &[(String, String)],
    mask: bool,
    out: &mut O,
    err: &mut E,
) -> Result<i32> {
    let program = &command[0];
    let mut cmd = Command::new(program);
    cmd.args(&command[1..]);
    // The child inherits this process's environment; secrets override variables of the same name
    for (key, value) in env_vars {
        cmd.env(key, value);
    }

    if !mask {
        let status = cmd
            .status()
            .with_context(|| format!("Failed to execute command: {program}"))?;
        return Ok(exit_code(status));
    }

    let mut values: Vec<Vec<u8>> = env_vars
        .iter()
        .map(|(_, v)| v.as_bytes().to_vec())
        .filter(|v| v.len() >= MIN_MASK_LEN)
        .collect();
    // Longest first, so a secret that contains another is masked whole
    values.sort_by_key(|v| std::cmp::Reverse(v.len()));
    values.dedup();

    let mut child = cmd
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .spawn()
        .with_context(|| format!("Failed to execute command: {program}"))?;
    let child_out = child.stdout.take().expect("stdout is piped");
    let child_err = child.stderr.take().expect("stderr is piped");

    let status = thread::scope(|scope| -> Result<ExitStatus> {
        let values_ref = &values;
        let err_thread = scope.spawn(move || copy_masked(child_err, err, values_ref));
        copy_masked(child_out, out, &values)?;
        err_thread.join().expect("stderr copier panicked")?;
        Ok(child.wait()?)
    })?;

    Ok(exit_code(status))
}

/// Copies line by line so a value is never split across two writes, masking each line.
fn copy_masked<R: Read, W: Write>(reader: R, writer: &mut W, values: &[Vec<u8>]) -> io::Result<()> {
    let mut reader = BufReader::new(reader);
    let mut line = Vec::new();
    loop {
        line.clear();
        if reader.read_until(b'\n', &mut line)? == 0 {
            break;
        }
        writer.write_all(&mask_bytes(&line, values))?;
        writer.flush()?;
    }
    Ok(())
}

/// Replaces every occurrence of each value (longest first) with [MASKED].
pub fn mask_bytes(input: &[u8], values: &[Vec<u8>]) -> Vec<u8> {
    let mut current = input.to_vec();
    for value in values {
        if value.is_empty() || current.len() < value.len() {
            continue;
        }
        let mut result = Vec::with_capacity(current.len());
        let mut i = 0;
        while i < current.len() {
            if current[i..].starts_with(value) {
                result.extend_from_slice(MASK);
                i += value.len();
            } else {
                result.push(current[i]);
                i += 1;
            }
        }
        current = result;
    }
    current
}

/// The child's exit code; a child killed by a signal exits with 128 + the signal number, as
/// shells report it.
pub fn exit_code(status: ExitStatus) -> i32 {
    if let Some(code) = status.code() {
        return code;
    }
    #[cfg(unix)]
    {
        use std::os::unix::process::ExitStatusExt;
        if let Some(signal) = status.signal() {
            return 128 + signal;
        }
    }
    1
}

#[cfg(test)]
mod tests {
    use super::*;

    fn vars(items: &[(&str, &str)]) -> Vec<(String, String)> {
        items
            .iter()
            .map(|(k, v)| (k.to_string(), v.to_string()))
            .collect()
    }

    #[test]
    fn masks_longest_values_first_and_leaves_short_ones() {
        let values = vec![b"sk_live_12345".to_vec(), b"sk_live".to_vec()];
        assert_eq!(
            mask_bytes(b"key=sk_live_12345 prefix=sk_live\n", &values),
            b"key=[MASKED] prefix=[MASKED]\n".to_vec()
        );
        assert_eq!(
            mask_bytes(b"nothing here", &values),
            b"nothing here".to_vec()
        );
    }

    #[cfg(unix)]
    #[test]
    fn passes_secrets_in_the_environment_and_returns_the_exit_code() {
        let env = vars(&[("CLI_TEST_TOKEN", "super-secret-value")]);
        let mut out = Vec::new();
        let mut err = Vec::new();
        let code = run_child(
            &[
                "sh".into(),
                "-c".into(),
                "echo token=$CLI_TEST_TOKEN; echo oops $CLI_TEST_TOKEN >&2; exit 7".into(),
            ],
            &env,
            true,
            &mut out,
            &mut err,
        )
        .unwrap();
        assert_eq!(code, 7);
        assert_eq!(String::from_utf8(out).unwrap(), "token=[MASKED]\n");
        assert_eq!(String::from_utf8(err).unwrap(), "oops [MASKED]\n");
    }

    #[cfg(unix)]
    #[test]
    fn unmasked_runs_inherit_output_and_report_signals() {
        let mut out = Vec::new();
        let mut err = Vec::new();
        let code = run_child(
            &["sh".into(), "-c".into(), "kill -TERM $$".into()],
            &[],
            false,
            &mut out,
            &mut err,
        )
        .unwrap();
        assert_eq!(code, 128 + 15);

        let missing = run_child(
            &["definitely-not-a-command-xyz".into()],
            &[],
            false,
            &mut out,
            &mut err,
        );
        assert!(missing.is_err());
    }
}
