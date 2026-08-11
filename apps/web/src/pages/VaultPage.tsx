import { useState, useCallback, useEffect } from 'react';
import { useParams, Link } from 'react-router-dom';
import { AppLayout } from '@/components/AppLayout';
import { Panel, Button, Input } from '@/components/win95';
import { ConfirmDialog } from '@/components/ConfirmDialog';
import { SecretFormDialog } from '@/components/SecretFormDialog';
import { EnvironmentFormDialog } from '@/components/EnvironmentFormDialog';
import { RevealSecretDialog } from '@/components/RevealSecretDialog';
import { EmptyState } from '@/components/EmptyState';
import { RoleBadge } from '@/components/RoleBadge';
import { PermissionGate, usePermission } from '@/components/PermissionGate';
import { LoadingState } from '@/components/LoadingState';
import { ErrorMessage } from '@/components/ErrorMessage';
import { useVault } from '@/hooks/use-vaults';
import {
  useEnvironments,
  useCreateEnvironment,
  useDeleteEnvironment,
} from '@/hooks/use-environments';
import {
  useSecrets,
  useCreateSecret,
  useUpdateSecret,
  useDeleteSecret,
  useRevealSecret,
} from '@/hooks/use-secrets';
import {
  Eye,
  EyeOff,
  Pencil,
  Trash2,
  AlertTriangle,
  ChevronLeft,
  Plus,
  Layers,
  Users,
  RefreshCw,
  Clock,
  Tag,
  Copy,
  Check,
  Loader2,
} from 'lucide-react';
import type { Secret, Environment, EnvironmentName } from '@/types/api';

const formatDate = (dateStr: string) => {
  const date = new Date(dateStr);
  return date.toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  });
};

const formatDateTime = (dateStr: string) => {
  const date = new Date(dateStr);
  return date.toLocaleString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
};

export const VaultPage = () => {
  const { id } = useParams<{ id: string }>();
  const [activeEnv, setActiveEnv] = useState<EnvironmentName>('dev');
  const [filter, setFilter] = useState('');
  const [selectedSecrets, setSelectedSecrets] = useState<Set<string>>(new Set());

  // Dialog states
  const [revealSecret, setRevealSecret] = useState<Secret | null>(null);
  const [revealedValue, setRevealedValue] = useState<string | null>(null);
  const [deleteConfirm, setDeleteConfirm] = useState<string | null>(null);
  const [editSecret, setEditSecret] = useState<Secret | null>(null);
  const [showCreateSecret, setShowCreateSecret] = useState(false);
  const [showCreateEnv, setShowCreateEnv] = useState(false);

  // Copy to clipboard state
  const [copyingSecretId, setCopyingSecretId] = useState<string | null>(null);
  const [copiedSecretId, setCopiedSecretId] = useState<string | null>(null);

  // Fetch vault details
  const { data: vault, isLoading: vaultLoading, error: vaultError } = useVault(id || '');

  // Fetch environments for this vault
  const { data: environments = [], isLoading: envsLoading } = useEnvironments(id || '');

  // Set activeEnv to first available environment when environments load
  useEffect(() => {
    if (environments.length > 0 && !environments.find((e) => e.name === activeEnv)) {
      setActiveEnv(environments[0].name);
    }
  }, [environments, activeEnv]);

  // Get current environment ID from activeEnv state
  const currentEnvId = environments.find((e) => e.name === activeEnv)?.id;

  // Fetch secrets for the current environment
  const { data: allSecrets = [], isLoading: secretsLoading } = useSecrets(currentEnvId || '');

  const { canCreate, canEdit, canDelete, canReveal, canManageMembers } = usePermission(
    vault?.userRole || 'viewer',
  );

  // Mutations
  const createEnvMutation = useCreateEnvironment();
  const deleteSecretMutation = useDeleteSecret();
  const revealMutation = useRevealSecret();
  const createSecretMutation = useCreateSecret();
  const updateSecretMutation = useUpdateSecret();

  // Filter secrets
  const secrets = allSecrets.filter(
    (s) =>
      s.keyName.toLowerCase().includes(filter.toLowerCase()) ||
      s.description?.toLowerCase().includes(filter.toLowerCase()),
  );

  const existingEnvs = environments.map((e) => e.name);

  const handleReveal = useCallback(async () => {
    if (!revealSecret) return;

    try {
      const response = await revealMutation.mutateAsync(revealSecret.id);
      setRevealedValue(response.value);
    } catch (error) {
      console.error('Failed to reveal secret:', error);
    }
  }, [revealSecret, revealMutation]);

  const handleDelete = useCallback(
    (secretId: string) => {
      deleteSecretMutation.mutate(secretId, {
        onSuccess: () => {
          setDeleteConfirm(null);
        },
        onError: (error) => {
          console.error('Failed to delete secret:', error);
          setDeleteConfirm(null);
        },
      });
    },
    [deleteSecretMutation],
  );

  const handleBulkDelete = useCallback(() => {
    console.log('Bulk delete:', Array.from(selectedSecrets));
    setSelectedSecrets(new Set());
  }, [selectedSecrets]);

  const handleCopySecret = useCallback(
    async (secretId: string) => {
      setCopyingSecretId(secretId);
      try {
        const response = await revealMutation.mutateAsync(secretId);
        await navigator.clipboard.writeText(response.value);
        setCopiedSecretId(secretId);
        setTimeout(() => setCopiedSecretId(null), 2000);
      } catch (error) {
        console.error('Failed to copy secret:', error);
      } finally {
        setCopyingSecretId(null);
      }
    },
    [revealMutation],
  );

  const toggleSecretSelection = (secretId: string) => {
    setSelectedSecrets((prev) => {
      const next = new Set(prev);
      if (next.has(secretId)) {
        next.delete(secretId);
      } else {
        next.add(secretId);
      }
      return next;
    });
  };

  const toggleAllSecrets = () => {
    if (selectedSecrets.size === secrets.length) {
      setSelectedSecrets(new Set());
    } else {
      setSelectedSecrets(new Set(secrets.map((s) => s.id)));
    }
  };

  // Loading state
  if (vaultLoading || envsLoading) {
    return (
      <AppLayout>
        <div className="space-y-win-sm">
          <div className="win-border-raised bg-background p-2">
            <div className="flex items-center gap-2 mb-2">
              <Link
                to="/vaults"
                className="text-info hover:underline text-win-body flex items-center gap-1"
              >
                <ChevronLeft size={12} strokeWidth={1.5} />
                Vaults
              </Link>
            </div>
            <LoadingState type="detail" rows={3} />
          </div>
          <Panel>
            <LoadingState type="table" rows={5} columns={6} />
          </Panel>
        </div>
      </AppLayout>
    );
  }

  // Error state
  if (vaultError || !vault) {
    return (
      <AppLayout>
        <div className="space-y-win-sm">
          <div className="win-border-raised bg-background p-2">
            <Link
              to="/vaults"
              className="text-info hover:underline text-win-body flex items-center gap-1"
            >
              <ChevronLeft size={12} strokeWidth={1.5} />
              Vaults
            </Link>
          </div>
          <ErrorMessage error={vaultError || new Error('Vault not found')} />
        </div>
      </AppLayout>
    );
  }

  // No environments state - set default to first available environment
  if (environments.length > 0 && !currentEnvId) {
    setActiveEnv(environments[0].name);
  }

  return (
    <AppLayout>
      <div className="space-y-win-sm">
        {/* Breadcrumb & Vault Header */}
        <div className="win-border-raised bg-background p-2">
          <div className="flex items-center gap-2 mb-2">
            <Link
              to="/vaults"
              className="text-info hover:underline text-win-body flex items-center gap-1"
            >
              <ChevronLeft size={12} strokeWidth={1.5} />
              Vaults
            </Link>
            <span className="text-muted-foreground">/</span>
            <span className="text-win-title font-semibold">{vault.name}</span>
            <RoleBadge role={vault.userRole} size="sm" />
          </div>

          <div className="flex items-start justify-between">
            <div>
              <p className="text-win-body text-muted-foreground">{vault.description}</p>
              <p className="text-win-small text-muted-foreground mt-1">
                Created by {vault.createdBy} on {formatDate(vault.createdAt)} —{' '}
                {vault.organizationName}
              </p>
            </div>
            <div className="flex gap-1">
              <PermissionGate permission="canManageMembers" userRole={vault.userRole}>
                <Link to={`/vaults/${id}/access`}>
                  <Button className="!min-w-0 flex items-center gap-1">
                    <Users size={12} strokeWidth={1.5} />
                    Members
                  </Button>
                </Link>
              </PermissionGate>
              <PermissionGate permission="canWrite" userRole={vault.userRole}>
                <Button className="!min-w-0 flex items-center gap-1">
                  <Pencil size={12} strokeWidth={1.5} />
                  Edit
                </Button>
              </PermissionGate>
            </div>
          </div>
        </div>

        {/* Environment Tabs */}
        <div className="win-border-raised bg-background">
          {environments.length === 0 ? (
            <div className="p-2">
              <EmptyState
                type="environments"
                actionLabel={canCreate ? 'Add First Environment' : undefined}
                onAction={canCreate ? () => setShowCreateEnv(true) : undefined}
              />
            </div>
          ) : (
            <>
              <div className="flex border-b border-border">
                {environments.map((env) => (
                  <button
                    key={env.id}
                    onClick={() => setActiveEnv(env.name)}
                    className={`px-4 py-2 text-win-body border-r border-border flex items-center gap-2 ${
                      activeEnv === env.name
                        ? 'bg-background font-semibold border-b-2 border-b-primary'
                        : 'bg-secondary hover:bg-background'
                    } ${env.name === 'prod' ? 'text-warning' : ''}`}
                  >
                    <Layers size={12} strokeWidth={1.5} />
                    {env.name.toUpperCase()}
                    <span className="text-win-small text-muted-foreground">
                      ({env.secretCount || 0})
                    </span>
                  </button>
                ))}
                <PermissionGate permission="canWrite" userRole={vault?.userRole || 'viewer'}>
                  <button
                    onClick={() => setShowCreateEnv(true)}
                    className="px-3 py-2 text-win-body bg-secondary hover:bg-background flex items-center gap-1 text-info"
                  >
                    <Plus size={12} strokeWidth={1.5} />
                    Add
                  </button>
                </PermissionGate>
              </div>

              {/* Production Warning */}
              {activeEnv === 'prod' && (
                <div className="bg-warning/10 border-b-2 border-warning px-3 py-2 flex items-center gap-2">
                  <AlertTriangle size={14} className="text-warning" strokeWidth={1.5} />
                  <span className="text-win-body">
                    <strong>PRODUCTION ENVIRONMENT</strong> — Changes affect live systems. All
                    actions are logged.
                  </span>
                </div>
              )}

              {/* Toolbar */}
              <div className="p-2 flex items-center gap-2 border-b border-border">
                <Input
                  placeholder="Filter secrets..."
                  value={filter}
                  onChange={(e) => setFilter(e.target.value)}
                  className="w-[200px]"
                />

                {selectedSecrets.size > 0 && (
                  <div className="flex items-center gap-2 ml-2">
                    <span className="text-win-small text-muted-foreground">
                      {selectedSecrets.size} selected
                    </span>
                    <PermissionGate permission="canDelete" userRole={vault?.userRole || 'viewer'}>
                      <Button
                        className="!min-w-0 flex items-center gap-1 text-warning"
                        onClick={handleBulkDelete}
                      >
                        <Trash2 size={12} strokeWidth={1.5} />
                        Delete
                      </Button>
                    </PermissionGate>
                  </div>
                )}

                <div className="flex-1" />

                <PermissionGate permission="canWrite" userRole={vault?.userRole || 'viewer'}>
                  <Button
                    className="!min-w-0 flex items-center gap-1"
                    onClick={() => setShowCreateSecret(true)}
                    disabled={!currentEnvId}
                  >
                    <Plus size={12} strokeWidth={1.5} />
                    Add Secret
                  </Button>
                </PermissionGate>
              </div>

              {/* Secrets Table */}
              {secretsLoading ? (
                <div className="m-2">
                  <LoadingState type="table" rows={3} columns={6} />
                </div>
              ) : secrets.length === 0 ? (
                <div className="m-2">
                  {currentEnvId ? (
                    <EmptyState
                      type="secrets"
                      actionLabel={canCreate ? 'Add First Secret' : undefined}
                      onAction={canCreate ? () => setShowCreateSecret(true) : undefined}
                    />
                  ) : (
                    <EmptyState
                      type="environments"
                      actionLabel={canCreate ? 'Create Environment First' : undefined}
                      onAction={canCreate ? () => setShowCreateEnv(true) : undefined}
                    />
                  )}
                </div>
              ) : (
                <div className="win-border-sunken bg-input m-2">
                  <table className="w-full text-win-body">
                    <thead>
                      <tr className="bg-secondary border-b border-border">
                        <th className="w-[30px] px-2 py-1">
                          <input
                            type="checkbox"
                            checked={selectedSecrets.size === secrets.length && secrets.length > 0}
                            onChange={toggleAllSecrets}
                            className="win-checkbox"
                          />
                        </th>
                        <th className="text-left px-2 py-1 font-semibold">Name</th>
                        <th className="text-left px-2 py-1 font-semibold w-[160px]">
                          Last Updated
                        </th>
                        <th className="text-left px-2 py-1 font-semibold w-[100px]">Rotation</th>
                        <th className="text-left px-2 py-1 font-semibold w-[100px]">Expires</th>
                        <th className="text-left px-2 py-1 font-semibold w-[80px]">Labels</th>
                        <th className="w-[120px]">Actions</th>
                      </tr>
                    </thead>
                    <tbody>
                      {secrets.map((secret, idx) => {
                        const isExpiringSoon =
                          secret.expiresAt &&
                          new Date(secret.expiresAt) <
                            new Date(Date.now() + 14 * 24 * 60 * 60 * 1000);
                        const needsRotation =
                          secret.rotationPolicy?.nextRotationAt &&
                          new Date(secret.rotationPolicy.nextRotationAt) <
                            new Date(Date.now() + 7 * 24 * 60 * 60 * 1000);

                        return (
                          <tr
                            key={secret.id}
                            className={`border-b border-border/50 ${
                              idx % 2 === 1 ? 'bg-background' : ''
                            } ${selectedSecrets.has(secret.id) ? 'bg-primary/10' : ''}`}
                          >
                            <td className="px-2 py-1">
                              <input
                                type="checkbox"
                                checked={selectedSecrets.has(secret.id)}
                                onChange={() => toggleSecretSelection(secret.id)}
                                className="win-checkbox"
                              />
                            </td>
                            <td className="px-2 py-1">
                              <div className="font-mono font-semibold">{secret.keyName}</div>
                              {secret.description && (
                                <div className="text-win-small text-muted-foreground">
                                  {secret.description}
                                </div>
                              )}
                            </td>
                            <td className="px-2 py-1 text-win-small">
                              <div>
                                {formatDateTime(secret.lastUpdatedAt ?? secret.updatedAt ?? '')}
                              </div>
                              <div className="text-muted-foreground">{secret.lastUpdatedBy}</div>
                            </td>
                            <td className="px-2 py-1">
                              {secret.rotationPolicy ? (
                                <div
                                  className={`flex items-center gap-1 ${needsRotation ? 'text-warning' : ''}`}
                                >
                                  <RefreshCw size={10} strokeWidth={1.5} />
                                  <span>{secret.rotationPolicy.intervalDays}d</span>
                                </div>
                              ) : (
                                <span className="text-muted-foreground">—</span>
                              )}
                            </td>
                            <td
                              className={`px-2 py-1 ${isExpiringSoon ? 'text-warning font-semibold' : ''}`}
                            >
                              {secret.expiresAt ? (
                                <div className="flex items-center gap-1">
                                  <Clock size={10} strokeWidth={1.5} />
                                  <span>{formatDate(secret.expiresAt)}</span>
                                </div>
                              ) : (
                                <span className="text-muted-foreground">—</span>
                              )}
                            </td>
                            <td className="px-2 py-1">
                              {secret.metadata && Object.keys(secret.metadata).length > 0 ? (
                                <div className="flex items-center gap-1">
                                  <Tag size={10} strokeWidth={1.5} />
                                  <span>{Object.keys(secret.metadata).length}</span>
                                </div>
                              ) : (
                                <span className="text-muted-foreground">—</span>
                              )}
                            </td>
                            <td className="px-2 py-1">
                              <div className="flex gap-1">
                                <PermissionGate
                                  permission="canReveal"
                                  userRole={vault?.userRole || 'viewer'}
                                >
                                  <button
                                    onClick={() => {
                                      setRevealSecret(secret);
                                      setRevealedValue(null);
                                    }}
                                    className="win-button !min-w-0 !px-1 !py-[2px]"
                                    title="Reveal"
                                  >
                                    <Eye size={12} />
                                  </button>
                                </PermissionGate>
                                <PermissionGate
                                  permission="canReveal"
                                  userRole={vault?.userRole || 'viewer'}
                                >
                                  <button
                                    onClick={() => handleCopySecret(secret.id)}
                                    disabled={copyingSecretId === secret.id}
                                    className="win-button !min-w-0 !px-1 !py-[2px]"
                                    title={
                                      copiedSecretId === secret.id ? 'Copied!' : 'Copy to clipboard'
                                    }
                                  >
                                    {copyingSecretId === secret.id ? (
                                      <Loader2 size={12} className="animate-spin" />
                                    ) : copiedSecretId === secret.id ? (
                                      <Check size={12} className="text-success" />
                                    ) : (
                                      <Copy size={12} />
                                    )}
                                  </button>
                                </PermissionGate>
                                <PermissionGate
                                  permission="canWrite"
                                  userRole={vault?.userRole || 'viewer'}
                                >
                                  <button
                                    onClick={() => setEditSecret(secret)}
                                    className="win-button !min-w-0 !px-1 !py-[2px]"
                                    title="Edit"
                                  >
                                    <Pencil size={12} />
                                  </button>
                                </PermissionGate>
                                <PermissionGate
                                  permission="canDelete"
                                  userRole={vault?.userRole || 'viewer'}
                                >
                                  <button
                                    onClick={() => setDeleteConfirm(secret.id)}
                                    className="win-button !min-w-0 !px-1 !py-[2px]"
                                    title="Delete"
                                  >
                                    <Trash2 size={12} />
                                  </button>
                                </PermissionGate>
                              </div>
                            </td>
                          </tr>
                        );
                      })}
                    </tbody>
                  </table>
                </div>
              )}

              {/* Status Bar */}
              <div className="px-2 py-1 border-t border-border text-win-small text-muted-foreground flex justify-between">
                <span>
                  {secrets.length} secret(s) in {activeEnv} environment
                </span>
                <span>Last sync: just now</span>
              </div>
            </>
          )}
        </div>
      </div>

      {/* Reveal Secret Dialog */}
      <RevealSecretDialog
        isOpen={!!revealSecret}
        secretName={revealSecret?.keyName || ''}
        secretValue={revealedValue}
        onClose={() => {
          setRevealSecret(null);
          setRevealedValue(null);
        }}
        onReveal={handleReveal}
      />

      {/* Delete Confirmation Dialog */}
      <ConfirmDialog
        isOpen={!!deleteConfirm}
        title="Delete Secret"
        message="Are you sure you want to permanently delete this secret? This action cannot be undone and will be logged."
        type="error"
        confirmLabel="Delete"
        onConfirm={() => deleteConfirm && handleDelete(deleteConfirm)}
        onCancel={() => setDeleteConfirm(null)}
      />

      {/* Create/Edit Secret Dialog */}
      <SecretFormDialog
        isOpen={showCreateSecret || !!editSecret}
        secret={editSecret}
        onClose={() => {
          setShowCreateSecret(false);
          setEditSecret(null);
        }}
        onSave={(data) => {
          if (!currentEnvId) return;

          if (editSecret) {
            // Update existing secret
            updateSecretMutation.mutate(
              { id: editSecret.id, data },
              {
                onSuccess: () => {
                  setEditSecret(null);
                },
                onError: (error) => {
                  console.error('Failed to update secret:', error);
                },
              },
            );
          } else {
            // Create new secret
            createSecretMutation.mutate(
              { envId: currentEnvId, data },
              {
                onSuccess: () => {
                  setShowCreateSecret(false);
                },
                onError: (error) => {
                  console.error('Failed to create secret:', error);
                },
              },
            );
          }
        }}
      />

      {/* Create Environment Dialog */}
      <EnvironmentFormDialog
        isOpen={showCreateEnv}
        existingEnvs={existingEnvs}
        onClose={() => setShowCreateEnv(false)}
        onSave={(data) => {
          if (!id) return;
          createEnvMutation.mutate(
            { vaultId: id, data },
            {
              onSuccess: () => {
                setShowCreateEnv(false);
              },
              onError: (error) => {
                console.error('Failed to create environment:', error);
              },
            },
          );
        }}
      />
    </AppLayout>
  );
};

export default VaultPage;
