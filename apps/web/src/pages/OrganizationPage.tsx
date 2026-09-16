import { useEffect, useMemo, useState } from 'react';
import { AppLayout } from '@/components/AppLayout';
import { Button, Input, Panel, Select } from '@/components/win95';
import { ConfirmDialog } from '@/components/ConfirmDialog';
import { InviteMemberDialog } from '@/components/InviteMemberDialog';
import { LoadingState } from '@/components/LoadingState';
import { ErrorMessage } from '@/components/ErrorMessage';
import { RoleBadge } from '@/components/RoleBadge';
import { useAuth } from '@/contexts/AuthContext';
import { useCurrentOrganization } from '@/contexts/OrganizationContext';
import { useToast } from '@/hooks/use-toast';
import {
  useInvites,
  useOrganizationMembers,
  useOrganizations,
  useRemoveOrganizationMember,
  useRenameOrganization,
  useRevokeInvite,
  useUpdateOrganizationMember,
} from '@/hooks/use-organizations';
import { Building, Mail, Pencil, Plus, Trash2, Users, X } from 'lucide-react';
import type { OrganizationMember, OrganizationRole } from '@/types/api';
import { useDocumentTitle } from '@/hooks/use-document-title';

const ROLE_OPTIONS: { value: OrganizationRole; label: string }[] = [
  { value: 'owner', label: 'Owner' },
  { value: 'admin', label: 'Admin' },
  { value: 'developer', label: 'Developer' },
  { value: 'oncall', label: 'On-Call' },
  { value: 'viewer', label: 'Viewer' },
];

const formatDate = (value: string | null) =>
  value ? new Date(value).toLocaleDateString(undefined, { dateStyle: 'medium' }) : 'Never';

const describeError = (err: unknown) => (err instanceof Error ? err.message : 'An error occurred');

export const OrganizationPage = () => {
  useDocumentTitle('Organization');
  const { user, refreshUser } = useAuth();
  const { toast } = useToast();
  const { error } = useOrganizations();
  const { currentOrganization: org, isLoading } = useCurrentOrganization();
  const canManage = org?.role === 'owner' || org?.role === 'admin';

  const { data: members = [], isLoading: membersLoading } = useOrganizationMembers(org?.id);
  const { data: invites = [] } = useInvites(org?.id, canManage);
  const rename = useRenameOrganization(org?.id ?? '');
  const updateMember = useUpdateOrganizationMember(org?.id ?? '');
  const removeMember = useRemoveOrganizationMember(org?.id ?? '');
  const revokeInvite = useRevokeInvite(org?.id ?? '');

  const [editingName, setEditingName] = useState(false);
  const [name, setName] = useState('');
  const [showInvite, setShowInvite] = useState(false);
  const [removing, setRemoving] = useState<OrganizationMember | null>(null);

  useEffect(() => {
    document.title = 'Organization — Vault Console';
  }, []);

  const roleOptions = useMemo(
    () => ROLE_OPTIONS.filter((r) => r.value !== 'owner' || org?.role === 'owner'),
    [org?.role],
  );

  if (isLoading) {
    return (
      <AppLayout>
        <LoadingState type="page" message="Loading organization..." />
      </AppLayout>
    );
  }
  if (error || !org) {
    return (
      <AppLayout>
        <ErrorMessage
          error={error ?? new Error('You are not a member of any organization')}
          action="load this organization"
        />
      </AppLayout>
    );
  }

  const saveName = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      await rename.mutateAsync(name);
      await refreshUser();
      setEditingName(false);
      toast({ title: 'Organization renamed' });
    } catch (err) {
      toast({ title: 'Could not rename', description: describeError(err), variant: 'destructive' });
    }
  };

  const changeRole = async (member: OrganizationMember, role: OrganizationRole) => {
    try {
      await updateMember.mutateAsync({ userId: member.userId, role });
      toast({ title: 'Role updated', description: `${member.email} is now ${role}.` });
    } catch (err) {
      toast({
        title: 'Could not change role',
        description: describeError(err),
        variant: 'destructive',
      });
    }
  };

  const confirmRemove = async () => {
    if (!removing) return;
    try {
      await removeMember.mutateAsync(removing.userId);
      toast({
        title: 'Member removed',
        description: `${removing.email} no longer has access to ${org.name} or its vaults.`,
      });
    } catch (err) {
      toast({ title: 'Could not remove', description: describeError(err), variant: 'destructive' });
    } finally {
      setRemoving(null);
    }
  };

  const revoke = async (inviteId: string, email: string) => {
    try {
      await revokeInvite.mutateAsync(inviteId);
      toast({
        title: 'Invitation revoked',
        description: `The link sent to ${email} no longer works.`,
      });
    } catch (err) {
      toast({ title: 'Could not revoke', description: describeError(err), variant: 'destructive' });
    }
  };

  // Only owners can touch another owner's role; nobody changes their own role here.
  const canEditMember = (member: OrganizationMember) =>
    canManage && member.userId !== user?.id && (member.role !== 'owner' || org.role === 'owner');

  return (
    <AppLayout>
      <div className="space-y-win-sm">
        {/* Header */}
        <div className="win-border-raised bg-background p-2 flex items-center justify-between gap-2 flex-wrap">
          <div className="flex items-center gap-2 min-w-0">
            <Building size={16} strokeWidth={1.5} />
            {editingName ? (
              <form onSubmit={saveName} className="flex items-center gap-1">
                <label htmlFor="org-name" className="sr-only">
                  Organization name
                </label>
                <Input
                  id="org-name"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  maxLength={100}
                  autoFocus
                  className="w-[260px]"
                />
                <Button type="submit" disabled={rename.isPending || !name.trim()}>
                  Save
                </Button>
                <Button type="button" onClick={() => setEditingName(false)}>
                  Cancel
                </Button>
              </form>
            ) : (
              <>
                <h1 className="text-win-title font-semibold truncate">{org.name}</h1>
                <RoleBadge role={org.role} size="sm" />
                {canManage && (
                  <button
                    className="win-button !min-w-0 !px-1 !py-[2px]"
                    title="Rename organization"
                    aria-label="Rename organization"
                    onClick={() => {
                      setName(org.name);
                      setEditingName(true);
                    }}
                  >
                    <Pencil size={12} />
                  </button>
                )}
              </>
            )}
          </div>
          <div className="flex items-center gap-2">
            {canManage && (
              <Button className="flex items-center gap-1" onClick={() => setShowInvite(true)}>
                <Plus size={12} strokeWidth={1.5} />
                Invite People
              </Button>
            )}
          </div>
        </div>

        {/* Members */}
        <Panel>
          <div className="flex items-center gap-2 mb-2">
            <Users size={14} strokeWidth={1.5} />
            <h2 className="text-win-section font-semibold">Members</h2>
            <span className="text-win-small text-muted-foreground">
              {members.length} {members.length === 1 ? 'person' : 'people'} · {org.vaultCount}{' '}
              {org.vaultCount === 1 ? 'vault' : 'vaults'}
            </span>
          </div>
          <div className="win-border-sunken bg-input overflow-x-auto">
            {membersLoading ? (
              <LoadingState type="inline" message="Loading members..." />
            ) : (
              <table className="w-full min-w-[560px] text-win-body">
                <thead>
                  <tr className="bg-secondary border-b border-border">
                    <th className="text-left px-2 py-1 font-semibold">Member</th>
                    <th className="text-left px-2 py-1 font-semibold w-[160px]">Role</th>
                    <th className="text-left px-2 py-1 font-semibold w-[120px]">Joined</th>
                    <th className="text-left px-2 py-1 font-semibold w-[120px]">Last login</th>
                    <th className="w-[60px]">
                      <span className="sr-only">Actions</span>
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {members.map((member, idx) => (
                    <tr
                      key={member.userId}
                      className={`border-b border-border/50 ${idx % 2 === 1 ? 'bg-background' : ''}`}
                    >
                      <td className="px-2 py-1">
                        <div>{member.email}</div>
                        <div className="text-win-small text-muted-foreground">
                          {member.name}
                          {member.userId === user?.id ? ' (you)' : ''}
                        </div>
                      </td>
                      <td className="px-2 py-1">
                        {canEditMember(member) ? (
                          <Select
                            aria-label={`Role for ${member.email}`}
                            value={member.role}
                            onChange={(e) => changeRole(member, e.target.value as OrganizationRole)}
                            options={roleOptions}
                            className="!py-0"
                            disabled={updateMember.isPending}
                          />
                        ) : (
                          <RoleBadge role={member.role} size="sm" />
                        )}
                      </td>
                      <td className="px-2 py-1 text-win-small">{formatDate(member.joinedAt)}</td>
                      <td className="px-2 py-1 text-win-small">{formatDate(member.lastLoginAt)}</td>
                      <td className="px-2 py-1">
                        {canEditMember(member) && (
                          <button
                            onClick={() => setRemoving(member)}
                            className="win-button !min-w-0 !px-1 !py-[2px]"
                            title="Remove from organization"
                            aria-label={`Remove ${member.email} from organization`}
                            disabled={removeMember.isPending}
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

        {/* Pending invitations */}
        {canManage && (
          <Panel>
            <div className="flex items-center gap-2 mb-2">
              <Mail size={14} strokeWidth={1.5} />
              <h2 className="text-win-section font-semibold">Pending Invitations</h2>
            </div>
            <div className="win-border-sunken bg-input overflow-x-auto">
              {invites.length === 0 ? (
                <div className="px-2 py-3 text-center text-win-body text-muted-foreground">
                  No pending invitations
                </div>
              ) : (
                <table className="w-full min-w-[560px] text-win-body">
                  <thead>
                    <tr className="bg-secondary border-b border-border">
                      <th className="text-left px-2 py-1 font-semibold">Email</th>
                      <th className="text-left px-2 py-1 font-semibold w-[120px]">Role</th>
                      <th className="text-left px-2 py-1 font-semibold w-[160px]">Invited by</th>
                      <th className="text-left px-2 py-1 font-semibold w-[120px]">Expires</th>
                      <th className="w-[60px]">
                        <span className="sr-only">Actions</span>
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    {invites.map((invite) => (
                      <tr key={invite.id} className="border-b border-border/50">
                        <td className="px-2 py-1">{invite.email}</td>
                        <td className="px-2 py-1">
                          <RoleBadge role={invite.role} size="sm" />
                        </td>
                        <td className="px-2 py-1 text-win-small">{invite.invitedBy || '—'}</td>
                        <td className="px-2 py-1 text-win-small">{formatDate(invite.expiresAt)}</td>
                        <td className="px-2 py-1">
                          <button
                            onClick={() => revoke(invite.id, invite.email)}
                            className="win-button !min-w-0 !px-1 !py-[2px]"
                            title="Revoke invitation"
                            aria-label={`Revoke invitation for ${invite.email}`}
                            disabled={revokeInvite.isPending}
                          >
                            <X size={12} />
                          </button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
            </div>
          </Panel>
        )}
      </div>

      <InviteMemberDialog
        isOpen={showInvite}
        organizationId={org.id}
        organizationName={org.name}
        inviterRole={org.role}
        onClose={() => setShowInvite(false)}
      />
      <ConfirmDialog
        isOpen={!!removing}
        title="Remove Member"
        message={`Removing this member takes away their access to every vault in ${org.name}.`}
        confirmLabel="Remove"
        confirmText={removing?.email}
        confirmTextLabel="email address"
        onConfirm={confirmRemove}
        onCancel={() => setRemoving(null)}
      />
    </AppLayout>
  );
};

export default OrganizationPage;
