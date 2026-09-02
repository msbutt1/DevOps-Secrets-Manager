import { useState } from 'react';
import { Monitor, AlertTriangle } from 'lucide-react';
import { Button, Panel } from '@/components/win95';
import { useRevokeOtherSessions, useRevokeSession, useSessions } from '@/hooks/use-sessions';
import { describeUserAgent, formatRelativeTime } from '@/lib/format';

/** Lists the user's signed-in sessions and lets them sign out others. */
export const SessionsPanel = () => {
  const { data: sessions = [], isLoading, error } = useSessions();
  const revokeSession = useRevokeSession();
  const revokeOthers = useRevokeOtherSessions();
  const [confirmOthers, setConfirmOthers] = useState(false);
  const [notice, setNotice] = useState<string | null>(null);

  const others = sessions.filter((s) => !s.current);
  const failure = revokeSession.error ?? revokeOthers.error;

  const signOutOthers = async () => {
    const result = await revokeOthers.mutateAsync();
    setConfirmOthers(false);
    setNotice(
      `Signed out ${result.sessionsRevoked} other session${result.sessionsRevoked === 1 ? '' : 's'}.`,
    );
  };

  return (
    <Panel>
      <div className="flex items-center justify-between gap-2 mb-3">
        <div className="flex items-center gap-2">
          <Monitor size={14} strokeWidth={1.5} />
          <h2 className="text-win-section font-semibold">Sessions</h2>
        </div>
        {others.length > 0 &&
          (confirmOthers ? (
            <div className="flex items-center gap-2">
              <span className="text-win-small">Sign out {others.length} other?</span>
              <Button onClick={signOutOthers} disabled={revokeOthers.isPending}>
                Yes
              </Button>
              <Button onClick={() => setConfirmOthers(false)}>No</Button>
            </div>
          ) : (
            <Button onClick={() => setConfirmOthers(true)}>Sign Out Other Sessions</Button>
          ))}
      </div>

      <p className="text-win-small text-muted-foreground mb-2">
        Each login is a session. Signing one out stops it from staying signed in; it may keep access
        for up to 15 minutes.
      </p>

      {(error || failure) && (
        <div className="win-border-sunken bg-background p-2 mb-2 flex items-center gap-2">
          <AlertTriangle size={14} className="text-warning" strokeWidth={1.5} />
          <span className="text-win-small text-warning">
            {(error ?? failure)?.message ?? 'Something went wrong'}
          </span>
        </div>
      )}
      {notice && <p className="text-win-small text-success mb-2">{notice}</p>}

      <div className="win-border-sunken bg-input overflow-x-auto">
        <table className="w-full text-win-body">
          <thead>
            <tr className="bg-background text-left">
              <th className="px-2 py-1 font-semibold">Client</th>
              <th className="px-2 py-1 font-semibold">IP Address</th>
              <th className="px-2 py-1 font-semibold">Signed In</th>
              <th className="px-2 py-1 font-semibold">Last Active</th>
              <th className="px-2 py-1" />
            </tr>
          </thead>
          <tbody>
            {isLoading && (
              <tr>
                <td colSpan={5} className="px-2 py-2 text-center text-muted-foreground">
                  Loading sessions...
                </td>
              </tr>
            )}
            {sessions.map((session) => (
              <tr key={session.id} className="border-t border-border/50">
                <td className="px-2 py-1" title={session.userAgent ?? undefined}>
                  {describeUserAgent(session.userAgent)}
                  {session.current && (
                    <span className="ml-2 text-win-small font-semibold text-success">
                      This session
                    </span>
                  )}
                </td>
                <td className="px-2 py-1 font-mono text-win-small">
                  {session.ipAddress ?? 'Unknown'}
                </td>
                <td className="px-2 py-1 text-win-small">
                  {new Date(session.createdAt).toLocaleString()}
                </td>
                <td className="px-2 py-1 text-win-small">
                  {formatRelativeTime(session.lastUsedAt)}
                </td>
                <td className="px-2 py-1 text-right">
                  {!session.current && (
                    <Button
                      onClick={() => revokeSession.mutate(session.id)}
                      disabled={revokeSession.isPending}
                      aria-label={`Sign out ${describeUserAgent(session.userAgent)} session`}
                    >
                      Sign Out
                    </Button>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </Panel>
  );
};
