import { useState } from 'react';
import { useDialog } from '@/hooks/use-dialog';
import { History, AlertTriangle } from 'lucide-react';
import { Button } from '@/components/win95';
import { useRestoreSecretVersion, useSecretVersions } from '@/hooks/use-secrets';
import { secretsApi } from '@/lib/api-client';
import type { Secret } from '@/types/api';

interface SecretHistoryDialogProps {
  secret: Secret;
  canReveal: boolean;
  canRestore: boolean;
  onClose: () => void;
}

/** Lists a secret's versions; earlier values can be revealed (audited) or restored. */
export const SecretHistoryDialog = ({
  secret,
  canReveal,
  canRestore,
  onClose,
}: SecretHistoryDialogProps) => {
  const dialogRef = useDialog<HTMLDivElement>(true, onClose);
  const { data: versions = [], isLoading, error } = useSecretVersions(secret.id);
  const restore = useRestoreSecretVersion();
  const [revealed, setRevealed] = useState<{ version: number; value: string } | null>(null);
  const [confirmRestore, setConfirmRestore] = useState<number | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  const reveal = async (version: number) => {
    setActionError(null);
    try {
      const result = await secretsApi.revealVersion(secret.id, version);
      setRevealed({ version, value: result.value });
    } catch (err) {
      setActionError(err instanceof Error ? err.message : 'Could not reveal the value');
    }
  };

  const doRestore = async (version: number) => {
    setActionError(null);
    try {
      const updated = await restore.mutateAsync({ id: secret.id, version });
      setNotice(`Version ${version} restored as version ${updated.version}.`);
      setRevealed(null);
    } catch (err) {
      setActionError(err instanceof Error ? err.message : 'Could not restore the value');
    } finally {
      setConfirmRestore(null);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      <div className="absolute inset-0 bg-foreground/20" onClick={onClose} />
      <div
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        className="relative z-10 win-border-raised bg-background w-full max-w-[560px] max-h-[90vh] overflow-auto"
        aria-labelledby="secret-history-title"
      >
        <div className="win-title-bar">
          <div className="flex items-center gap-2">
            <History size={14} />
            <span id="secret-history-title">History — {secret.keyName}</span>
          </div>
          <button onClick={onClose} className="text-win-small px-2" aria-label="Close">
            X
          </button>
        </div>

        <div className="p-3 space-y-2">
          <p className="text-win-small text-muted-foreground">
            Every value change is kept. Restoring an earlier value adds it as a new version, so
            nothing is lost. Reveals and restores are recorded in the audit log.
          </p>

          {(error || actionError) && (
            <div className="win-border-sunken bg-background p-2 flex items-center gap-2">
              <AlertTriangle size={14} className="text-warning" strokeWidth={1.5} />
              <span className="text-win-small text-warning">{error?.message ?? actionError}</span>
            </div>
          )}
          {notice && (
            <p className="text-win-small text-success" role="status">
              {notice}
            </p>
          )}

          <div className="win-border-sunken bg-input max-h-[320px] overflow-auto">
            <table className="w-full text-win-body">
              <thead>
                <tr className="bg-background text-left">
                  <th className="px-2 py-1 font-semibold">Version</th>
                  <th className="px-2 py-1 font-semibold">Set</th>
                  <th className="px-2 py-1 font-semibold">By</th>
                  <th className="px-2 py-1" />
                </tr>
              </thead>
              <tbody>
                {isLoading && (
                  <tr>
                    <td colSpan={4} className="px-2 py-2 text-center text-muted-foreground">
                      Loading history...
                    </td>
                  </tr>
                )}
                {versions.map((v) => (
                  <tr key={v.version} className="border-t border-border/50 align-top">
                    <td className="px-2 py-1 font-mono">
                      v{v.version}
                      {v.current && (
                        <span className="ml-2 text-win-small font-semibold text-success">
                          current
                        </span>
                      )}
                      {v.restoredFrom && (
                        <div className="text-win-small text-muted-foreground">
                          restored from v{v.restoredFrom}
                        </div>
                      )}
                      {revealed?.version === v.version && (
                        <div className="mt-1 font-mono text-win-small break-all bg-background px-1">
                          {revealed.value}
                        </div>
                      )}
                    </td>
                    <td className="px-2 py-1 text-win-small">
                      {new Date(v.createdAt).toLocaleString()}
                    </td>
                    <td className="px-2 py-1 text-win-small">{v.createdBy || 'Unknown'}</td>
                    <td className="px-2 py-1">
                      <div className="flex gap-1 justify-end">
                        {canReveal &&
                          (revealed?.version === v.version ? (
                            <Button onClick={() => setRevealed(null)}>Hide</Button>
                          ) : (
                            <Button onClick={() => reveal(v.version)}>Reveal</Button>
                          ))}
                        {canRestore &&
                          !v.current &&
                          (confirmRestore === v.version ? (
                            <Button
                              onClick={() => doRestore(v.version)}
                              disabled={restore.isPending}
                            >
                              Confirm
                            </Button>
                          ) : (
                            <Button onClick={() => setConfirmRestore(v.version)}>Restore</Button>
                          ))}
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          <div className="flex justify-end pt-2 border-t border-border">
            <Button onClick={onClose}>Close</Button>
          </div>
        </div>
      </div>
    </div>
  );
};
