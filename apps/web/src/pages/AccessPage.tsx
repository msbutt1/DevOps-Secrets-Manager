import { useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import { AppLayout } from '@/components/AppLayout';
import { Panel, Button, Input, Select } from '@/components/win95';
import { ConfirmDialog } from '@/components/ConfirmDialog';
import { ChevronLeft, Plus, Trash2 } from 'lucide-react';
import { useToast } from '@/hooks/use-toast';
import { useVaultMembers, useAddMember, useUpdateMember, useRemoveMember } from '@/hooks/use-access';
import { useVault } from '@/hooks/use-vaults';
import type { VaultMember, VaultRole } from '@/types/api';

const roleOptions: { value: VaultRole; label: string }[] = [
  { value: 'admin', label: 'Admin' },
  { value: 'developer', label: 'Developer' },
  { value: 'oncall', label: 'On-Call' },
  { value: 'viewer', label: 'Viewer' },
];

const permissionLabels: Record<keyof VaultMember['permissions'], string> = {
  canRead: 'Read',
  canWrite: 'Write',
  canReveal: 'Reveal',
  canManageMembers: 'Members',
  canDelete: 'Delete',
};

export const AccessPage = () => {
  const { id } = useParams<{ id: string }>();
  const { toast } = useToast();
  const { data: vault } = useVault(id || '');
  const { data: members = [], isLoading } = useVaultMembers(id || '');
  const addMemberMutation = useAddMember();
  const updateMemberMutation = useUpdateMember();
  const removeMemberMutation = useRemoveMember();

  const [newEmail, setNewEmail] = useState('');
  const [newRole, setNewRole] = useState<VaultRole>('developer');
  const [deleteConfirm, setDeleteConfirm] = useState<string | null>(null);
  const [showAddForm, setShowAddForm] = useState(false);

  const handleAddMember = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!id) return;

    try {
      await addMemberMutation.mutateAsync({
        vaultId: id,
        data: { email: newEmail, role: newRole },
      });
      toast({
        title: 'Member added',
        description: `${newEmail} has been added to the vault.`,
      });
      setNewEmail('');
      setNewRole('developer');
      setShowAddForm(false);
    } catch (error) {
      toast({
        title: 'Failed to add member',
        description: error instanceof Error ? error.message : 'An error occurred',
        variant: 'destructive',
      });
    }
  };

  const handleRoleChange = async (userId: string, role: VaultRole) => {
    if (!id) return;

    try {
      await updateMemberMutation.mutateAsync({
        vaultId: id,
        userId,
        data: { role },
      });
      toast({
        title: 'Role updated',
        description: 'Member role has been updated successfully.',
      });
    } catch (error) {
      toast({
        title: 'Failed to update role',
        description: error instanceof Error ? error.message : 'An error occurred',
        variant: 'destructive',
      });
    }
  };

  const handleRemove = async (userId: string) => {
    if (!id) return;

    try {
      await removeMemberMutation.mutateAsync({ vaultId: id, userId });
      toast({
        title: 'Member removed',
        description: 'Member has been removed from the vault.',
      });
      setDeleteConfirm(null);
    } catch (error) {
      toast({
        title: 'Failed to remove member',
        description: error instanceof Error ? error.message : 'An error occurred',
        variant: 'destructive',
      });
    }
  };

  return (
    <AppLayout>
      <div className="space-y-win-sm">
        {/* Breadcrumb */}
        <div className="win-border-raised bg-background p-2 flex items-center gap-2">
          <Link to="/" className="text-info hover:underline text-win-body flex items-center gap-1">
            <ChevronLeft size={12} strokeWidth={1.5} />
            Dashboard
          </Link>
          <span className="text-muted-foreground">/</span>
          <Link to={`/vaults/${id}`} className="text-info hover:underline text-win-body">
            {vault?.name || 'Loading...'}
          </Link>
          <span className="text-muted-foreground">/</span>
          <span className="text-win-title font-semibold">Access Management</span>
        </div>

        <div className="flex gap-win-sm">
          {/* Members Panel */}
          <div className="flex-1">
            <Panel>
              <div className="flex items-center justify-between mb-2">
                <h2 className="text-win-section font-semibold">Vault Members</h2>
                <Button 
                  className="!min-w-0 flex items-center gap-1"
                  onClick={() => setShowAddForm(!showAddForm)}
                >
                  <Plus size={12} strokeWidth={1.5} />
                  Add Member
                </Button>
              </div>

              {/* Add Member Form */}
              {showAddForm && (
                <form onSubmit={handleAddMember} className="win-border-groove p-2 mb-2">
                  <div className="flex gap-2 items-end">
                    <div className="flex-1">
                      <label className="block text-win-body mb-1">Email:</label>
                      <Input
                        type="email"
                        value={newEmail}
                        onChange={(e) => setNewEmail(e.target.value)}
                        placeholder="user@company.com"
                        required
                        disabled={addMemberMutation.isPending}
                      />
                    </div>
                    <div className="w-[140px]">
                      <label className="block text-win-body mb-1">Role:</label>
                      <Select
                        value={newRole}
                        onChange={(e) => setNewRole(e.target.value as VaultRole)}
                        options={roleOptions}
                        disabled={addMemberMutation.isPending}
                      />
                    </div>
                    <Button type="submit" disabled={addMemberMutation.isPending}>
                      {addMemberMutation.isPending ? 'Adding...' : 'Add'}
                    </Button>
                    <Button type="button" onClick={() => setShowAddForm(false)} disabled={addMemberMutation.isPending}>
                      Cancel
                    </Button>
                  </div>
                </form>
              )}

              {/* Members Table */}
              <div className="win-border-sunken bg-input">
                {isLoading ? (
                  <div className="p-4 text-center text-muted-foreground">
                    Loading members...
                  </div>
                ) : members.length === 0 ? (
                  <div className="p-4 text-center text-muted-foreground">
                    No members found. Add a member to get started.
                  </div>
                ) : (
                  <table className="w-full text-win-body">
                    <thead>
                      <tr className="bg-secondary border-b border-border">
                        <th className="text-left px-2 py-1 font-semibold">Email</th>
                        <th className="text-left px-2 py-1 font-semibold w-[140px]">Role</th>
                        <th className="text-left px-2 py-1 font-semibold w-[200px]">Permissions</th>
                        <th className="w-[60px]">Actions</th>
                      </tr>
                    </thead>
                    <tbody>
                      {members.map((member, idx) => (
                        <tr
                          key={member.userId}
                          className={`border-b border-border/50 ${
                            idx % 2 === 1 ? 'bg-background' : ''
                          }`}
                        >
                          <td className="px-2 py-1">
                            <div>{member.email}</div>
                            <div className="text-win-small text-muted-foreground">{member.name}</div>
                          </td>
                          <td className="px-2 py-1">
                            {member.role === 'owner' ? (
                              <span className="text-info font-semibold">Owner</span>
                            ) : (
                              <Select
                                value={member.role}
                                onChange={(e) => handleRoleChange(member.userId, e.target.value as VaultRole)}
                                options={roleOptions}
                                className="!py-0"
                                disabled={updateMemberMutation.isPending}
                              />
                            )}
                          </td>
                          <td className="px-2 py-1">
                            <div className="flex gap-2 flex-wrap">
                              {Object.entries(member.permissions).map(([key, value]) => (
                                <span
                                  key={key}
                                  className={`text-win-small px-1 ${
                                    value ? 'bg-success/10 text-success' : 'text-muted-foreground'
                                  }`}
                                >
                                  {value ? '✓' : '—'} {permissionLabels[key as keyof VaultMember['permissions']]}
                                </span>
                              ))}
                            </div>
                          </td>
                          <td className="px-2 py-1">
                            {member.role !== 'owner' && (
                              <button
                                onClick={() => setDeleteConfirm(member.userId)}
                                className="win-button !min-w-0 !px-1 !py-[2px]"
                                title="Remove"
                                disabled={removeMemberMutation.isPending}
                              >
                                <Trash2 size={12} />
                              </button>
                            )}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                )}
              </div>
            </Panel>
          </div>

          {/* Permissions Reference Panel */}
          <div className="w-[280px]">
            <Panel>
              <h2 className="text-win-section font-semibold mb-2">Role Permissions Reference</h2>
              
              <div className="space-y-2 text-win-body">
                <div className="border-b border-border pb-2">
                  <strong>Owner</strong>
                  <p className="text-win-small text-muted-foreground">
                    Full control. Can delete vault, manage all members, reveal secrets.
                  </p>
                </div>
                <div className="border-b border-border pb-2">
                  <strong>Admin</strong>
                  <p className="text-win-small text-muted-foreground">
                    Can manage members (except owner). Can reveal secrets and modify values.
                  </p>
                </div>
                <div className="border-b border-border pb-2">
                  <strong>Developer</strong>
                  <p className="text-win-small text-muted-foreground">
                    Can read and write secrets. Cannot reveal values or manage members.
                  </p>
                </div>
                <div className="border-b border-border pb-2">
                  <strong>On-Call</strong>
                  <p className="text-win-small text-muted-foreground">
                    Can read and reveal secrets. Cannot modify values.
                  </p>
                </div>
                <div>
                  <strong>Viewer</strong>
                  <p className="text-win-small text-muted-foreground">
                    Read-only. Cannot reveal or modify secrets.
                  </p>
                </div>
              </div>
            </Panel>
          </div>
        </div>
      </div>

      {/* Remove Confirmation Dialog */}
      <ConfirmDialog
        isOpen={!!deleteConfirm}
        title="Remove Member"
        message="Are you sure you want to remove this member from the vault? They will lose all access immediately."
        type="warning"
        confirmLabel="Remove"
        onConfirm={() => deleteConfirm && handleRemove(deleteConfirm)}
        onCancel={() => setDeleteConfirm(null)}
      />
    </AppLayout>
  );
};

export default AccessPage;
