import { useState } from 'react';
import { Link } from 'react-router-dom';
import { AppLayout } from '@/components/AppLayout';
import { Panel, Button, Input } from '@/components/win95';
import { VaultFormDialog } from '@/components/VaultFormDialog';
import { ConfirmDialog } from '@/components/ConfirmDialog';
import { EmptyState } from '@/components/EmptyState';
import { RoleBadge } from '@/components/RoleBadge';
import { PermissionGate } from '@/components/PermissionGate';
import { LoadingState } from '@/components/LoadingState';
import { ErrorMessage } from '@/components/ErrorMessage';
import { useVaults, useCreateVault, useUpdateVault, useDeleteVault } from '@/hooks/use-vaults';
import { useCurrentOrganization } from '@/contexts/OrganizationContext';
import { canCreateVaults } from '@/lib/organizations';
import { Database, Plus, ChevronRight, Search, Pencil, Trash2, Layers, Key } from 'lucide-react';
import type { Vault, VaultCreateRequest, VaultUpdateRequest } from '@/types/api';

export const VaultsListPage = () => {
  const [searchQuery, setSearchQuery] = useState('');
  const [showCreateDialog, setShowCreateDialog] = useState(false);
  const [editVault, setEditVault] = useState<Vault | null>(null);
  const [deleteConfirm, setDeleteConfirm] = useState<string | null>(null);

  // Fetch vaults
  const { currentOrganization } = useCurrentOrganization();
  const {
    data: vaults = [],
    isLoading,
    error,
    refetch,
    isFetching,
  } = useVaults(currentOrganization?.id);

  // Mutations
  const createVault = useCreateVault();
  const updateVault = useUpdateVault();
  const deleteVault = useDeleteVault();

  // Creating vaults depends on the role in the current organization
  const canCreateVault = canCreateVaults(currentOrganization?.role);

  const filteredVaults = vaults.filter(
    (vault) =>
      vault.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
      vault.description?.toLowerCase().includes(searchQuery.toLowerCase()),
  );

  const handleCreate = (data: VaultCreateRequest) => {
    createVault.mutate(
      { ...data, organizationId: currentOrganization?.id },
      {
        onSuccess: () => {
          setShowCreateDialog(false);
        },
      },
    );
  };

  const handleEdit = (data: VaultUpdateRequest) => {
    if (!editVault) return;
    updateVault.mutate(
      { id: editVault.id, data },
      {
        onSuccess: () => {
          setEditVault(null);
        },
      },
    );
  };

  const handleDelete = (vaultId: string) => {
    deleteVault.mutate(vaultId, {
      onSuccess: () => {
        setDeleteConfirm(null);
      },
    });
  };

  const formatDate = (dateStr: string) => {
    return new Date(dateStr).toLocaleDateString('en-US', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
    });
  };

  // Loading state
  if (isLoading) {
    return (
      <AppLayout>
        <div className="space-y-win-sm">
          <div className="win-border-raised bg-background p-2">
            <h1 className="text-win-title font-semibold">Vault Registry</h1>
            <p className="text-win-body text-muted-foreground">Loading vaults...</p>
          </div>
          <Panel>
            <LoadingState type="table" rows={5} columns={7} />
          </Panel>
        </div>
      </AppLayout>
    );
  }

  // Error state
  if (error) {
    return (
      <AppLayout>
        <div className="space-y-win-sm">
          <div className="win-border-raised bg-background p-2">
            <h1 className="text-win-title font-semibold">Vault Registry</h1>
          </div>
          <ErrorMessage
            error={error}
            action="load your vaults"
            onRetry={() => refetch()}
            isRetrying={isFetching}
          />
        </div>
      </AppLayout>
    );
  }

  return (
    <AppLayout>
      <div className="space-y-win-sm">
        {/* Page Header */}
        <div className="win-border-raised bg-background p-2 flex items-center justify-between">
          <div>
            <h1 className="text-win-title font-semibold">Vault Registry</h1>
            <p className="text-win-body text-muted-foreground">
              All secret vaults accessible to your account
            </p>
          </div>
          {canCreateVault && (
            <Button className="flex items-center gap-1" onClick={() => setShowCreateDialog(true)}>
              <Plus size={12} strokeWidth={1.5} />
              New Vault
            </Button>
          )}
        </div>

        {/* Search & Filters */}
        <Panel>
          <div className="flex items-center gap-2">
            <div className="relative flex-1 max-w-[300px]">
              <Search
                size={12}
                className="absolute left-2 top-1/2 -translate-y-1/2 text-muted-foreground"
              />
              <Input
                placeholder="Search vaults..."
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                className="pl-7"
              />
            </div>
            <span className="text-win-small text-muted-foreground">
              {filteredVaults.length} of {vaults.length} vaults
            </span>
          </div>
        </Panel>

        {/* Vaults Table */}
        {filteredVaults.length === 0 ? (
          <EmptyState
            type="vaults"
            actionLabel={canCreateVault ? 'Create First Vault' : undefined}
            onAction={canCreateVault ? () => setShowCreateDialog(true) : undefined}
          />
        ) : (
          <Panel>
            <div className="flex items-center gap-2 mb-2">
              <Database size={14} strokeWidth={1.5} />
              <h2 className="text-win-section font-semibold">Available Vaults</h2>
            </div>

            <div className="win-border-sunken bg-input">
              <table className="w-full text-win-body">
                <thead>
                  <tr className="bg-secondary border-b border-border">
                    <th className="text-left px-2 py-1 font-semibold">Vault Name</th>
                    <th className="text-left px-2 py-1 font-semibold w-[120px]">Organization</th>
                    <th className="text-left px-2 py-1 font-semibold w-[100px]">Your Role</th>
                    <th className="text-left px-2 py-1 font-semibold w-[70px]">
                      <div className="flex items-center gap-1">
                        <Key size={10} strokeWidth={1.5} />
                        Secrets
                      </div>
                    </th>
                    <th className="text-left px-2 py-1 font-semibold w-[70px]">
                      <div className="flex items-center gap-1">
                        <Layers size={10} strokeWidth={1.5} />
                        Envs
                      </div>
                    </th>
                    <th className="text-left px-2 py-1 font-semibold w-[100px]">Updated</th>
                    <th className="w-[120px]"></th>
                  </tr>
                </thead>
                <tbody>
                  {filteredVaults.map((vault, idx) => (
                    <tr
                      key={vault.id}
                      className={`border-b border-border/50 hover:bg-primary/10 ${
                        idx % 2 === 1 ? 'bg-background' : ''
                      }`}
                    >
                      <td className="px-2 py-1">
                        <Link
                          to={`/vaults/${vault.id}`}
                          className="hover:underline text-info font-semibold"
                        >
                          {vault.name}
                        </Link>
                        {vault.description && (
                          <div className="text-win-small text-muted-foreground truncate max-w-[300px]">
                            {vault.description}
                          </div>
                        )}
                      </td>
                      <td className="px-2 py-1">{vault.organizationName}</td>
                      <td className="px-2 py-1">
                        <RoleBadge role={vault.userRole} size="sm" />
                      </td>
                      <td className="px-2 py-1 text-center">{vault.secretCount}</td>
                      <td className="px-2 py-1 text-center">{vault.envCount}</td>
                      <td className="px-2 py-1 text-win-small text-muted-foreground">
                        {formatDate(vault.updatedAt)}
                      </td>
                      <td className="px-2 py-1">
                        <div className="flex gap-1">
                          <PermissionGate permission="canWrite" userRole={vault.userRole}>
                            <button
                              onClick={() => setEditVault(vault)}
                              className="win-button !min-w-0 !px-1 !py-[2px]"
                              title="Edit"
                            >
                              <Pencil size={12} />
                            </button>
                          </PermissionGate>
                          <PermissionGate permission="canDelete" userRole={vault.userRole}>
                            <button
                              onClick={() => setDeleteConfirm(vault.id)}
                              className="win-button !min-w-0 !px-1 !py-[2px]"
                              title="Delete"
                            >
                              <Trash2 size={12} />
                            </button>
                          </PermissionGate>
                          <Link
                            to={`/vaults/${vault.id}`}
                            className="win-button !min-w-0 !px-2 !py-[2px] flex items-center gap-1"
                          >
                            Open
                            <ChevronRight size={12} strokeWidth={1.5} />
                          </Link>
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </Panel>
        )}
      </div>

      {/* Create/Edit Vault Dialog */}
      <VaultFormDialog
        isOpen={showCreateDialog || !!editVault}
        vault={editVault}
        onClose={() => {
          setShowCreateDialog(false);
          setEditVault(null);
        }}
        onSave={editVault ? handleEdit : handleCreate}
      />

      {/* Delete Confirmation Dialog */}
      <ConfirmDialog
        isOpen={!!deleteConfirm}
        title="Delete Vault"
        message={`Deleting this vault permanently removes its environments and every secret in them. This cannot be undone.`}
        type="error"
        confirmLabel="Delete Vault"
        confirmText={vaults.find((v) => v.id === deleteConfirm)?.name}
        confirmTextLabel="vault name"
        onConfirm={() => deleteConfirm && handleDelete(deleteConfirm)}
        onCancel={() => setDeleteConfirm(null)}
      />
    </AppLayout>
  );
};

export default VaultsListPage;
