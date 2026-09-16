import { useState } from 'react';
import { useDialog } from '@/hooks/use-dialog';
import { useQueryClient } from '@tanstack/react-query';
import { Copy, AlertTriangle } from 'lucide-react';
import { Button, Select } from '@/components/win95';
import { secretsApi } from '@/lib/api-client';
import { secretKeys, useSecrets } from '@/hooks/use-secrets';
import type { Environment, ImportResult } from '@/types/api';

interface CopySecretsDialogProps {
  environments: Environment[];
  targetEnvId: string;
  targetName: string;
  onClose: () => void;
}

/** Copies chosen keys from another environment of the same vault into this one. */
export const CopySecretsDialog = ({
  environments,
  targetEnvId,
  targetName,
  onClose,
}: CopySecretsDialogProps) => {
  const dialogRef = useDialog<HTMLDivElement>(true, onClose);
  const queryClient = useQueryClient();
  const sources = environments.filter((e) => e.id !== targetEnvId);
  const [sourceId, setSourceId] = useState(sources[0]?.id ?? '');
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [overwrite, setOverwrite] = useState(false);
  const [preview, setPreview] = useState<ImportResult | null>(null);
  const [done, setDone] = useState<ImportResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const { data: sourceSecrets = [], isLoading } = useSecrets(sourceId);

  const run = async (dryRun: boolean) => {
    setBusy(true);
    setError(null);
    try {
      const result = await secretsApi.copyFrom(targetEnvId, {
        sourceEnvironmentId: sourceId,
        keys: [...selected],
        overwrite,
        dryRun,
      });
      if (dryRun) {
        setPreview(result);
      } else {
        setDone(result);
        queryClient.invalidateQueries({ queryKey: secretKeys.all });
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Copy failed');
    } finally {
      setBusy(false);
    }
  };

  const toggle = (key: string) =>
    setSelected((current) => {
      const next = new Set(current);
      if (!next.delete(key)) next.add(key);
      setPreview(null);
      return next;
    });

  const result = done ?? preview;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      <div className="absolute inset-0 bg-foreground/20" onClick={onClose} />
      <div
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        className="relative z-10 win-border-raised bg-background w-full max-w-[520px] max-h-[90vh] overflow-auto"
        aria-labelledby="copy-secrets-title"
      >
        <div className="win-title-bar">
          <div className="flex items-center gap-2">
            <Copy size={14} />
            <span id="copy-secrets-title">Copy secrets into {targetName}</span>
          </div>
          <button onClick={onClose} className="text-win-small px-2" aria-label="Close">
            X
          </button>
        </div>

        <div className="p-3 space-y-2">
          {sources.length === 0 ? (
            <p className="text-win-body">This vault has no other environment to copy from.</p>
          ) : (
            <>
              <div>
                <label htmlFor="copy-source" className="block text-win-body mb-1">
                  Copy from:
                </label>
                <Select
                  id="copy-source"
                  value={sourceId}
                  onChange={(e) => {
                    setSourceId(e.target.value);
                    setSelected(new Set());
                    setPreview(null);
                  }}
                  disabled={!!done}
                  options={sources.map((env) => ({ value: env.id, label: env.name }))}
                />
              </div>

              <div className="win-border-sunken bg-input max-h-[180px] overflow-auto p-1">
                {isLoading && <p className="text-win-small text-muted-foreground">Loading...</p>}
                {!isLoading && sourceSecrets.length === 0 && (
                  <p className="text-win-small text-muted-foreground">
                    That environment has no secrets.
                  </p>
                )}
                {sourceSecrets.map((secret) => (
                  <label key={secret.id} className="flex items-center gap-2 text-win-body">
                    <input
                      type="checkbox"
                      className="win-checkbox"
                      checked={selected.has(secret.keyName)}
                      onChange={() => toggle(secret.keyName)}
                      disabled={!!done}
                    />
                    <span className="font-mono">{secret.keyName}</span>
                  </label>
                ))}
              </div>
              <p className="text-win-small text-muted-foreground">
                {selected.size === 0
                  ? 'Nothing selected: every key is copied.'
                  : `${selected.size} key(s) selected.`}{' '}
                Values are decrypted and encrypted again for this environment; both sides are
                recorded in the audit log.
              </p>

              <label className="flex items-center gap-1 text-win-body">
                <input
                  type="checkbox"
                  className="win-checkbox"
                  checked={overwrite}
                  onChange={(e) => {
                    setOverwrite(e.target.checked);
                    setPreview(null);
                  }}
                  disabled={!!done}
                />
                Overwrite keys that already exist here
              </label>
            </>
          )}

          {error && (
            <div className="win-border-sunken bg-background p-2 flex items-center gap-2">
              <AlertTriangle size={14} className="text-warning" strokeWidth={1.5} />
              <span className="text-win-small text-warning">{error}</span>
            </div>
          )}

          {result && (
            <div className="win-border-sunken bg-input p-2 space-y-1" role="status">
              {done && <p className="text-win-body font-semibold">Copy complete.</p>}
              {(
                [
                  [done ? 'Created' : 'Will create', result.created, 'text-success'],
                  [done ? 'Updated' : 'Will update', result.updated, 'text-warning'],
                  ['Unchanged (same value)', result.unchanged, 'text-muted-foreground'],
                  ['Skipped (already exist)', result.skipped, 'text-muted-foreground'],
                ] as [string, string[], string][]
              ).map(([label, keys, color]) =>
                keys.length > 0 ? (
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
              sources.length > 0 &&
              (preview ? (
                <Button
                  onClick={() => run(false)}
                  disabled={busy || preview.created.length + preview.updated.length === 0}
                >
                  {busy ? 'Copying...' : 'Copy'}
                </Button>
              ) : (
                <Button onClick={() => run(true)} disabled={busy || !sourceId}>
                  {busy ? 'Checking...' : 'Preview'}
                </Button>
              ))}
          </div>
        </div>
      </div>
    </div>
  );
};
