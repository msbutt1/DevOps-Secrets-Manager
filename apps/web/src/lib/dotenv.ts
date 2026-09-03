// Reads and writes .env files the same way as the CLI (apps/cli/src/utils/dotenv.rs), so a file
// exported from the web app imports with `secrets import` and the other way round.

export class DotenvError extends Error {
  constructor(
    readonly line: number,
    message: string,
  ) {
    super(`line ${line}: ${message}`);
    this.name = 'DotenvError';
  }
}

export type Pair = [key: string, value: string];

export const isValidKey = (key: string) => /^[A-Za-z_][A-Za-z0-9_]*$/.test(key);

/** Formats one KEY=value line, quoting only when needed. */
export function formatLine(key: string, value: string): string {
  if (/^[A-Za-z0-9_\-./:@%+,=]*$/.test(value)) return `${key}=${value}`;
  if (!/['\n\r]/.test(value)) return `${key}='${value}'`;
  const escaped = value
    .replace(/\\/g, '\\\\')
    .replace(/"/g, '\\"')
    .replace(/\n/g, '\\n')
    .replace(/\r/g, '\\r')
    .replace(/\t/g, '\\t');
  return `${key}="${escaped}"`;
}

export const formatDotenv = (pairs: Pair[]) =>
  pairs.map(([k, v]) => formatLine(k, v) + '\n').join('');

function closingQuote(s: string): number {
  let escaped = false;
  for (let i = 0; i < s.length; i++) {
    const c = s[i];
    if (c === '\\' && !escaped) escaped = true;
    else if (c === '"' && !escaped) return i;
    else escaped = false;
  }
  return -1;
}

function unescape(s: string): string {
  const map: Record<string, string> = { n: '\n', r: '\r', t: '\t', '"': '"', '\\': '\\' };
  let out = '';
  for (let i = 0; i < s.length; i++) {
    if (s[i] !== '\\') {
      out += s[i];
      continue;
    }
    const next = s[i + 1];
    if (next === undefined) out += '\\';
    else if (next in map) out += map[next];
    else out += '\\' + next;
    i++;
  }
  return out;
}

/**
 * Parses a .env document: blank lines and # comments are skipped, `export ` is allowed, later
 * duplicates override earlier ones, and double-quoted values may span lines.
 */
export function parseDotenv(content: string): Pair[] {
  const pairs: Pair[] = [];
  const lines = content.split(/\r?\n/);
  let i = 0;
  while (i < lines.length) {
    const lineNo = i + 1;
    let line = lines[i].trim();
    i++;
    if (!line || line.startsWith('#')) continue;
    if (line.startsWith('export ')) line = line.slice(7).trimStart();

    const eq = line.indexOf('=');
    if (eq < 0) throw new DotenvError(lineNo, 'expected KEY=value');
    const key = line.slice(0, eq).trim();
    if (!isValidKey(key)) throw new DotenvError(lineNo, `'${key}' is not a valid variable name`);
    const rest = line.slice(eq + 1).trimStart();

    let value: string;
    if (rest.startsWith("'")) {
      const end = rest.indexOf("'", 1);
      if (end < 0) throw new DotenvError(lineNo, 'unterminated single-quoted value');
      value = rest.slice(1, end);
    } else if (rest.startsWith('"')) {
      let raw = rest.slice(1);
      for (;;) {
        const end = closingQuote(raw);
        if (end >= 0) {
          raw = raw.slice(0, end);
          break;
        }
        if (i >= lines.length) throw new DotenvError(lineNo, 'unterminated double-quoted value');
        raw += '\n' + lines[i];
        i++;
      }
      value = unescape(raw);
    } else {
      const hash = rest.indexOf(' #');
      value = (hash >= 0 ? rest.slice(0, hash) : rest).trimEnd();
    }

    const existing = pairs.find(([k]) => k === key);
    if (existing) existing[1] = value;
    else pairs.push([key, value]);
  }
  return pairs;
}
