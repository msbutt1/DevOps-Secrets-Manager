import { useEffect, useState } from 'react';
import { Button, Input, Panel, Select } from '@/components/win95';
import { CopyButton } from '@/components/CopyButton';
import { AlertTriangle, KeyRound, Trash2 } from 'lucide-react';
import { useToast } from '@/hooks/use-toast';
import {
  useCreateServiceToken,
  useRevokeServiceToken,
  useServiceTokens,
} from '@/hooks/use-service-tokens';
import type { CreatedServiceToken } from '@/types/api';

interface ServiceTokensDialogProps {
  isOpen: boolean;
  vaultName: string;
  environmentId: string;
  environmentName: string;
  onClose: () => void;
}

const EXPIRY_OPTIONS = [
  { value: '30', label: '30 days' },
  { value: '90', label: '90 days' },
  { value: '365', label: '1 year' },
  { value: '', label: 'Never' },
];

const formatDate = (value: string | null) =>
  value ? new Date(value).toLocaleDateString(undefined, { dateStyle: 'medium' }) : '—';

export const ServiceTokensDialog = ({
  isOpen,
  vaultName,
  environmentId,
  environmentName,
  onClose,
}: ServiceTokensDialogProps) => {
  const { toast } = useToast();
  const { data: tokens = [], isLoading } = useServiceTokens(environmentId, isOpen);
  const createToken = useCreateServiceToken(environmentId);
  const revokeToken = useRevokeServiceToken(environmentId);
  const [name, setName] = useState('');
  const [expiry, setExpiry] = useState('90');
  const [created, setCreated] = useState<CreatedServiceToken | null>(null);

  useEffect(() => {
    if (isOpen) {
      setName('');
      setExpiry('90');
      setCreated(null);
    }
  }, [isOpen]);

  if (!isOpen) return null;

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      const token = await createToken.mutateAsync({
        name: name.trim(),
        expiresInDays: expiry ? Number(expiry) : null,
      });
      setCreated(token);
      setName('');
    } catch (err) {
      toast({
        title: 'Could not create token',
        description: err instanceof Error ? err.message : 'An error occurred',
        variant: 'destructive',
      });
    }
  };

  const handleRevoke = async (id: string, tokenName: string) => {
    try {
      await revokeToken.mutateAsync(id);
      if (created?.id === id) setCreated(null);
      toast({ title: 'Token revoked', description: `${tokenName} can no longer read secrets.` });
    } catch (err) {
      toast({
        title: 'Could not revoke token',
        description: err instanceof Error ? err.message : 'An error occurred',
        variant: 'destructive',
      });
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-2">
      <div className="absolute inset-0 bg-foreground/20" onClick={onClose} />
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="service-tokens-title"
        className="relative win-border-raised bg-background w-full max-w-[640px] max-h-[90vh] overflow-auto animate-win-open"
      >
        <div className="win-title-bar">
          <div className="flex items-center gap-2">
            <KeyRound size={14} />
            <span id="service-tokens-title">
              Service Tokens — {vaultName} / {environmentName}
            </span>
          </div>
        </div>

        <div className="p-3 space-y-3">
          <Panel className="!p-2">
            <p className="text-win-small">
              A service token can read every secret in <strong>{environmentName}</strong> and
              nothing else. Use it in CI with <code>SECRETS_TOKEN</code> and{' '}
              <code>secrets run -- your-command</code>. Every read is recorded in the audit log.
            </p>
          </Panel>

          {created && (
            <div role="status" className="win-border-sunken bg-input p-2 space-y-2">
              <div className="flex items-start gap-2">
                <AlertTriangle size={14} className="text-warning mt-[2px] flex-shrink-0" />
                <p className="text-win-body">
                  Copy <strong>{created.name}</strong> now. It will not be shown again.
                </p>
              </div>
              <div className="flex items-center gap-2">
                <code className="font-mono text-win-small break-all flex-1 bg-background p-1 border border-border">
                  {created.token}
                </code>
                <CopyButton value={created.token} variant="button" label="Copy token" />
              </div>
            </div>
          )}

          <form onSubmit={handleCreate} className="flex items-end gap-2 flex-wrap">
            <div className="flex-1 min-w-[180px]">
              <label htmlFor="token-name" className="block text-win-body mb-1">
                Token name:
              </label>
              <Input
                id="token-name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="e.g., github-actions-deploy"
                maxLength={100}
                disabled={createToken.isPending}
              />
            </div>
            <div className="w-[120px]">
              <label htmlFor="token-expiry" className="block text-win-body mb-1">
                Expires:
              </label>
              <Select
                id="token-expiry"
                value={expiry}
                onChange={(e) => setExpiry(e.target.value)}
                options={EXPIRY_OPTIONS}
                disabled={createToken.isPending}
              />
            </div>
            <Button type="submit" disabled={createToken.isPending || !name.trim()}>
              {createToken.isPending ? 'Creating...' : 'Create Token'}
            </Button>
          </form>

          <div className="win-border-sunken bg-input overflow-x-auto">
            {isLoading ? (
              <div className="px-2 py-3 text-center text-win-body text-muted-foreground">
                Loading tokens...
              </div>
            ) : tokens.length === 0 ? (
              <div className="px-2 py-3 text-center text-win-body text-muted-foreground">
                No active tokens for this environment
              </div>
            ) : (
              <table className="w-full text-win-body">
                <thead>
                  <tr className="bg-secondary border-b border-border">
                    <th className="text-left px-2 py-1 font-semibold">Name</th>
                    <th className="text-left px-2 py-1 font-semibold">Created</th>
                    <th className="text-left px-2 py-1 font-semibold">Expires</th>
                    <th className="text-left px-2 py-1 font-semibold">Last used</th>
                    <th className="w-[40px]">
                      <span className="sr-only">Actions</span>
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {tokens.map((token) => (
                    <tr key={token.id} className="border-b border-border/50">
                      <td className="px-2 py-1">
                        <div>{token.name}</div>
                        <div className="text-win-small text-muted-foreground font-mono">
                          {token.prefix}…
                        </div>
                      </td>
                      <td className="px-2 py-1 text-win-small">
                        {formatDate(token.createdAt)}
                        {token.createdBy && (
                          <div className="text-muted-foreground">{token.createdBy}</div>
                        )}
                      </td>
                      <td className="px-2 py-1 text-win-small">
                        {token.expiresAt ? formatDate(token.expiresAt) : 'Never'}
                      </td>
                      <td className="px-2 py-1 text-win-small">
                        {token.lastUsedAt ? formatDate(token.lastUsedAt) : 'Never'}
                      </td>
                      <td className="px-2 py-1">
                        <button
                          onClick={() => handleRevoke(token.id, token.name)}
                          className="win-button !min-w-0 !px-1 !py-[2px]"
                          title="Revoke token"
                          aria-label={`Revoke ${token.name}`}
                          disabled={revokeToken.isPending}
                        >
                          <Trash2 size={12} />
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>

          <div className="flex justify-end pt-2 border-t border-border">
            <Button onClick={onClose}>Close</Button>
          </div>
        </div>
      </div>
    </div>
  );
};
