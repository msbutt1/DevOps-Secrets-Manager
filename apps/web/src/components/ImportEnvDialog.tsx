import { useState } from 'react';
import { useDialog } from '@/hooks/use-dialog';
import { useQueryClient } from '@tanstack/react-query';
import { Upload, AlertTriangle } from 'lucide-react';
import { Button } from '@/components/win95';
import { secretsApi } from '@/lib/api-client';
import { parseDotenv, type Pair } from '@/lib/dotenv';
import { secretKeys } from '@/hooks/use-secrets';
import type { ImportResult } from '@/types/api';

interface ImportEnvDialogProps {
  envId: string;
  environmentName: string;
  onClose: () => void;
}

/** Paste or upload a .env file, preview what happens to each key, then import. */
export const ImportEnvDialog = ({ envId, environmentName, onClose }: ImportEnvDialogProps) => {
  const dialogRef = useDialog<HTMLDivElement>(true, onClose);
  const queryClient = useQueryClient();
  const [content, setContent] = useState('');
  const [overwrite, setOverwrite] = useState(false);
  const [pairs, setPairs] = useState<Pair[] | null>(null);
  const [preview, setPreview] = useState<ImportResult | null>(null);
  const [done, setDone] = useState<ImportResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const request = (dryRun: boolean, parsed: Pair[]) =>
    secretsApi.importEnvironment(envId, {
      secrets: parsed.map(([key, value]) => ({ key, value })),
      overwrite,
      dryRun,
    });

  const runPreview = async () => {
    setError(null);
    setPreview(null);
    let parsed: Pair[];
    try {
      parsed = parseDotenv(content);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Invalid .env file');
      return;
    }
    if (parsed.length === 0) {
      setError('No KEY=value lines found');
      return;
    }
    setBusy(true);
    try {
      setPairs(parsed);
      setPreview(await request(true, parsed));
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Preview failed');
    } finally {
      setBusy(false);
    }
  };

  const runImport = async () => {
    if (!pairs) return;
    setBusy(true);
    setError(null);
    try {
      setDone(await request(false, pairs));
      queryClient.invalidateQueries({ queryKey: secretKeys.all });
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Import failed');
    } finally {
      setBusy(false);
    }
  };

  const loadFile = async (file: File | undefined) => {
    if (!file) return;
    setContent(await file.text());
    setPreview(null);
  };

  const result = done ?? preview;
  const rows: [string, string[] | undefined, string][] = result
    ? [
        [done ? 'Created' : 'Will create', result.created, 'text-success'],
        [done ? 'Updated' : 'Will update', result.updated, 'text-warning'],
        ['Unchanged (same value)', result.unchanged, 'text-muted-foreground'],
        ['Skipped (already exist)', result.skipped, 'text-muted-foreground'],
      ]
    : [];

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      <div className="absolute inset-0 bg-foreground/20" onClick={onClose} />
      <div
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        className="relative z-10 win-border-raised bg-background w-full max-w-[560px] max-h-[90vh] overflow-auto"
        aria-labelledby="import-env-title"
      >
        <div className="win-title-bar">
          <div className="flex items-center gap-2">
            <Upload size={14} />
            <span id="import-env-title">Import .env into {environmentName}</span>
          </div>
          <button onClick={onClose} className="text-win-small px-2" aria-label="Close">
            X
          </button>
        </div>

        <div className="p-3 space-y-2">
          {!done && (
            <>
              <label htmlFor="env-content" className="block text-win-body">
                Paste a .env file or choose one:
              </label>
              <textarea
                id="env-content"
                className="win-input w-full h-[140px] font-mono text-win-small"
                value={content}
                onChange={(e) => {
                  setContent(e.target.value);
                  setPreview(null);
                }}
                placeholder={'DATABASE_URL=postgres://...\nAPI_KEY=...'}
                spellCheck={false}
                autoComplete="off"
              />
              <div className="flex flex-wrap items-center gap-3">
                <input
                  type="file"
                  accept=".env,text/plain"
                  aria-label="Choose a .env file"
                  className="text-win-small"
                  onChange={(e) => loadFile(e.target.files?.[0])}
                />
                <label className="flex items-center gap-1 text-win-body">
                  <input
                    type="checkbox"
                    className="win-checkbox"
                    checked={overwrite}
                    onChange={(e) => {
                      setOverwrite(e.target.checked);
                      setPreview(null);
                    }}
                  />
                  Overwrite existing keys
                </label>
              </div>
            </>
          )}

          {error && (
            <div className="win-border-sunken bg-background p-2 flex items-center gap-2">
              <AlertTriangle size={14} className="text-warning" strokeWidth={1.5} />
              <span className="text-win-small text-warning">{error}</span>
            </div>
          )}

          {result && (
            <div
              className="win-border-sunken bg-input p-2 space-y-1 max-h-[200px] overflow-auto"
              role="status"
            >
              {done && <p className="text-win-body font-semibold">Import complete.</p>}
              {rows.map(([label, keys, color]) =>
                keys && keys.length > 0 ? (
                  <div key={label} className="text-win-small">
                    <span className={`font-semibold ${color}`}>
                      {label} ({keys.length}):
                    </span>{' '}
                    <span className="font-mono">{keys.join(', ')}</span>
                  </div>
                ) : null,
              )}
            </div>
          )}

          <div className="flex justify-end gap-2 pt-2 border-t border-border">
            <Button onClick={onClose}>{done ? 'Close' : 'Cancel'}</Button>
            {!done &&
              (preview ? (
                <Button
                  onClick={runImport}
                  disabled={busy || preview.created.length + preview.updated.length === 0}
                >
                  {busy ? 'Importing...' : 'Import'}
                </Button>
              ) : (
                <Button onClick={runPreview} disabled={busy || !content.trim()}>
                  {busy ? 'Checking...' : 'Preview'}
                </Button>
              ))}
          </div>
        </div>
      </div>
    </div>
  );
};
