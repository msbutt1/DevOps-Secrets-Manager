import { useEffect, useState } from 'react';
import { Link, useLocation, useNavigate, useSearchParams } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Button, Panel } from '@/components/win95';
import { AlertTriangle, Loader2, Mail, Users } from 'lucide-react';
import { useAuth } from '@/contexts/AuthContext';
import { invitesApi } from '@/lib/api-client';
import { RoleBadge } from '@/components/RoleBadge';
import { useDocumentTitle } from '@/hooks/use-document-title';

/** Accept page for invitation links: /invite?token=... */
export const InvitePage = () => {
  useDocumentTitle('Invitation');
  const [searchParams] = useSearchParams();
  const token = searchParams.get('token') ?? '';
  const { user, isAuthenticated, isLoading: authLoading, logout, refreshUser } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const queryClient = useQueryClient();
  const [accepting, setAccepting] = useState(false);
  const [acceptError, setAcceptError] = useState<string | null>(null);

  const {
    data: invite,
    isLoading,
    error,
  } = useQuery({
    queryKey: ['invite-lookup', token],
    queryFn: () => invitesApi.lookup(token),
    enabled: !!token,
    retry: false,
  });

  useEffect(() => {
    document.title = 'Invitation — Vault Console';
  }, []);

  const emailMatches =
    !!user && !!invite && user.email.toLowerCase() === invite.email.toLowerCase();

  const accept = async () => {
    setAccepting(true);
    setAcceptError(null);
    try {
      await invitesApi.accept(token);
      await refreshUser();
      await queryClient.invalidateQueries();
      navigate('/', { replace: true });
    } catch (err) {
      setAcceptError(err instanceof Error ? err.message : 'Could not accept the invitation');
    } finally {
      setAccepting(false);
    }
  };

  let body;
  if (!token) {
    body = <Problem message="This link has no invitation token." />;
  } else if (isLoading || authLoading) {
    body = (
      <Panel>
        <div className="flex items-center justify-center gap-3 py-4">
          <Loader2 size={24} className="animate-spin text-info" strokeWidth={1.5} />
          <p className="text-win-body">Checking invitation...</p>
        </div>
      </Panel>
    );
  } else if (error || !invite) {
    body = (
      <Problem
        message={
          error instanceof Error
            ? error.message
            : 'This invitation is invalid, expired or already used.'
        }
      />
    );
  } else {
    body = (
      <>
        <Panel className="mb-3">
          <div className="flex items-start gap-3">
            <Users size={24} className="text-info flex-shrink-0" strokeWidth={1.5} />
            <div className="text-win-body space-y-1">
              <p>
                <strong>{invite.invitedBy || 'A team member'}</strong> invited{' '}
                <strong>{invite.email}</strong> to join <strong>{invite.organizationName}</strong>.
              </p>
              <p className="flex items-center gap-2">
                Role: <RoleBadge role={invite.role} />
              </p>
              <p className="text-win-small text-muted-foreground">
                Expires {new Date(invite.expiresAt).toLocaleString()}
              </p>
            </div>
          </div>
        </Panel>

        {acceptError && <Problem message={acceptError} />}

        <div className="flex justify-end gap-2 pt-2 border-t border-border">
          {isAuthenticated && emailMatches && (
            <Button onClick={accept} disabled={accepting}>
              {accepting ? 'Joining...' : 'Accept Invitation'}
            </Button>
          )}
          {isAuthenticated && !emailMatches && (
            <>
              <p className="text-win-small text-warning mr-auto self-center">
                You are logged in as {user?.email}. Log out and use {invite.email}.
              </p>
              <Button onClick={() => logout()}>Log Out</Button>
            </>
          )}
          {!isAuthenticated && invite.accountExists && (
            <Button
              onClick={() =>
                navigate('/login', {
                  state: { from: { pathname: location.pathname, search: location.search } },
                })
              }
            >
              Log In to Accept
            </Button>
          )}
          {!isAuthenticated && !invite.accountExists && (
            <Button onClick={() => navigate(`/register?invite=${encodeURIComponent(token)}`)}>
              Create Account
            </Button>
          )}
        </div>
      </>
    );
  }

  return (
    <div className="min-h-screen bg-[#008080] flex items-center justify-center p-4">
      <div className="win-border-raised bg-background w-full max-w-[460px]">
        <div className="win-title-bar">
          <div className="flex items-center gap-2">
            <Mail size={14} strokeWidth={1.5} />
            <span>Vault Console — Invitation</span>
          </div>
        </div>
        <div className="p-win-md">{body}</div>
        <div className="win-border-raised bg-background-secondary h-[18px] flex items-center px-2 border-t-0">
          <Link to="/" className="text-win-small text-info hover:underline">
            Go to Vault Console
          </Link>
        </div>
      </div>
    </div>
  );
};

const Problem = ({ message }: { message: string }) => (
  <div role="alert" className="win-border-sunken bg-background mb-3 p-2 flex items-start gap-2">
    <AlertTriangle size={16} className="text-warning flex-shrink-0 mt-[1px]" strokeWidth={1.5} />
    <p className="text-win-body">{message}</p>
  </div>
);

export default InvitePage;
