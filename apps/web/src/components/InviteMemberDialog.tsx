import { useEffect, useState } from 'react';
import { useDialog } from '@/hooks/use-dialog';
import { Button, Input, Panel, Select } from '@/components/win95';
import { Mail } from 'lucide-react';
import { useToast } from '@/hooks/use-toast';
import { useCreateInvite } from '@/hooks/use-organizations';
import type { OrganizationRole } from '@/types/api';

interface InviteMemberDialogProps {
  isOpen: boolean;
  organizationId: string;
  organizationName: string;
  /** The inviter's role; only owners can invite owners */
  inviterRole: OrganizationRole;
  onClose: () => void;
}

const ROLE_OPTIONS: { value: OrganizationRole; label: string }[] = [
  { value: 'viewer', label: 'Viewer' },
  { value: 'oncall', label: 'On-Call' },
  { value: 'developer', label: 'Developer' },
  { value: 'admin', label: 'Admin' },
  { value: 'owner', label: 'Owner' },
];

export const InviteMemberDialog = ({
  isOpen,
  organizationId,
  organizationName,
  inviterRole,
  onClose,
}: InviteMemberDialogProps) => {
  const dialogRef = useDialog<HTMLDivElement>(isOpen, onClose);
  const [email, setEmail] = useState('');
  const [role, setRole] = useState<OrganizationRole>('developer');
  const { toast } = useToast();
  const createInvite = useCreateInvite(organizationId);

  useEffect(() => {
    if (isOpen) {
      setEmail('');
      setRole('developer');
    }
  }, [isOpen]);

  if (!isOpen) return null;

  const roles = ROLE_OPTIONS.filter((r) => r.value !== 'owner' || inviterRole === 'owner');

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      const invite = await createInvite.mutateAsync({ email: email.trim(), role });
      toast({
        title: invite.emailSent ? 'Invitation sent' : 'Invitation created, email not sent',
        description: invite.emailSent
          ? `${invite.email} can join ${organizationName} within 7 days.`
          : 'Email delivery is not configured. Check the server logs or configure SMTP, then invite again.',
        variant: invite.emailSent ? undefined : 'destructive',
      });
      onClose();
    } catch (err) {
      toast({
        title: 'Could not invite',
        description: err instanceof Error ? err.message : 'An error occurred',
        variant: 'destructive',
      });
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      <div className="absolute inset-0 bg-foreground/20" onClick={onClose} />
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="invite-dialog-title"
        ref={dialogRef}
        className="relative z-10 win-border-raised bg-background w-full max-w-[400px] max-h-[90vh] overflow-auto animate-win-open"
      >
        <div className="win-title-bar">
          <div className="flex items-center gap-2">
            <Mail size={14} />
            <span id="invite-dialog-title">Invite to {organizationName}</span>
          </div>
        </div>
        <form onSubmit={handleSubmit} className="p-3 space-y-3">
          <Panel className="!p-2">
            <p className="text-win-small">
              We email a single-use link that expires in 7 days. After they join, add them to the
              vaults they need.
            </p>
          </Panel>
          <div>
            <label htmlFor="invite-email" className="block text-win-body mb-1">
              Email:
            </label>
            <Input
              id="invite-email"
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="teammate@example.com"
              required
              autoFocus
              disabled={createInvite.isPending}
            />
          </div>
          <div>
            <label htmlFor="invite-role" className="block text-win-body mb-1">
              Organization role:
            </label>
            <Select
              id="invite-role"
              value={role}
              onChange={(e) => setRole(e.target.value as OrganizationRole)}
              options={roles}
              disabled={createInvite.isPending}
            />
            <p className="text-win-small text-muted-foreground mt-1">
              Owners and admins can see every vault; other roles only see vaults they are added to.
            </p>
          </div>
          <div className="flex justify-end gap-2 pt-2 border-t border-border">
            <Button type="submit" disabled={createInvite.isPending || !email.trim()}>
              {createInvite.isPending ? 'Sending...' : 'Send Invitation'}
            </Button>
            <Button type="button" onClick={onClose} disabled={createInvite.isPending}>
              Cancel
            </Button>
          </div>
        </form>
      </div>
    </div>
  );
};
