import { useState } from 'react';
import { useAuth } from '@/contexts/AuthContext';
import { AppLayout } from '@/components/AppLayout';
import { Panel, Button, Input } from '@/components/win95';
import { User, Shield, Clock, Building, AlertTriangle, Check } from 'lucide-react';
import { authApi } from '@/lib/api-client';

export const SettingsPage = () => {
  const { user } = useAuth();
  const [showPasswordDialog, setShowPasswordDialog] = useState(false);
  const [currentPassword, setCurrentPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);

  const handleChangePassword = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    if (newPassword !== confirmPassword) {
      setError('New passwords do not match');
      return;
    }

    if (newPassword.length < 8) {
      setError('New password must be at least 8 characters');
      return;
    }

    setIsSubmitting(true);
    try {
      await authApi.changePassword({ currentPassword, newPassword });
      setSuccess(true);
      setCurrentPassword('');
      setNewPassword('');
      setConfirmPassword('');
      setTimeout(() => {
        setShowPasswordDialog(false);
        setSuccess(false);
      }, 2000);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to change password');
    } finally {
      setIsSubmitting(false);
    }
  };

  const closeDialog = () => {
    setShowPasswordDialog(false);
    setCurrentPassword('');
    setNewPassword('');
    setConfirmPassword('');
    setError(null);
    setSuccess(false);
  };

  const formatDate = (dateStr?: string) => {
    if (!dateStr) return 'N/A';
    return new Date(dateStr).toLocaleDateString('en-US', {
      year: 'numeric',
      month: 'long',
      day: 'numeric',
    });
  };

  return (
    <AppLayout>
      <div className="space-y-win-sm">
        {/* Page Header */}
        <div className="win-border-raised bg-background p-2">
          <h1 className="text-win-title font-semibold">User Settings</h1>
          <p className="text-win-body text-muted-foreground">Account information and preferences</p>
        </div>

        <div className="flex gap-win-sm">
          {/* Profile Panel */}
          <div className="flex-1">
            <Panel>
              <div className="flex items-center gap-2 mb-3">
                <User size={14} strokeWidth={1.5} />
                <h2 className="text-win-section font-semibold">Profile Information</h2>
              </div>

              <div className="space-y-3">
                <div className="win-border-groove p-2">
                  <div className="text-win-small text-muted-foreground mb-1">User ID</div>
                  <div className="font-mono text-win-body">{user?.id || 'N/A'}</div>
                </div>

                <div className="win-border-groove p-2">
                  <div className="text-win-small text-muted-foreground mb-1">Name</div>
                  <div className="text-win-body font-semibold">{user?.name || 'N/A'}</div>
                </div>

                <div className="win-border-groove p-2">
                  <div className="text-win-small text-muted-foreground mb-1">Email Address</div>
                  <div className="text-win-body">{user?.email || 'N/A'}</div>
                </div>

                <div className="win-border-groove p-2">
                  <div className="text-win-small text-muted-foreground mb-1">Account Created</div>
                  <div className="text-win-body">{formatDate(user?.createdAt)}</div>
                </div>
              </div>
            </Panel>
          </div>

          {/* Security & Organizations */}
          <div className="w-[320px] space-y-win-sm">
            {/* Security Panel */}
            <Panel>
              <div className="flex items-center gap-2 mb-3">
                <Shield size={14} strokeWidth={1.5} />
                <h2 className="text-win-section font-semibold">Security</h2>
              </div>

              <div className="space-y-2 text-win-body">
                <div className="flex justify-between items-center">
                  <span>Session Status:</span>
                  <span className="text-success font-semibold">Active</span>
                </div>
                <div className="flex justify-between items-center">
                  <span>Auth Method:</span>
                  <span className="text-muted-foreground">Email/Password</span>
                </div>
              </div>

              <div className="mt-3 pt-2 border-t border-border">
                <Button className="w-full" onClick={() => setShowPasswordDialog(true)}>
                  Change Password
                </Button>
              </div>
            </Panel>

            {/* Organizations Panel */}
            <Panel>
              <div className="flex items-center gap-2 mb-3">
                <Building size={14} strokeWidth={1.5} />
                <h2 className="text-win-section font-semibold">Organizations</h2>
              </div>

              <div className="win-border-sunken bg-input">
                {user?.organizations && user.organizations.length > 0 ? (
                  user.organizations.map((org, idx) => (
                    <div
                      key={org.id}
                      className={`px-2 py-1 border-b border-border/50 ${
                        idx % 2 === 1 ? 'bg-background' : ''
                      }`}
                    >
                      <div className="text-win-body font-semibold">{org.name}</div>
                      <div className="text-win-small text-muted-foreground">Role: {org.role}</div>
                    </div>
                  ))
                ) : (
                  <div className="px-2 py-2 text-win-body text-muted-foreground text-center">
                    No organizations
                  </div>
                )}
              </div>
            </Panel>

            {/* Session Info */}
            <Panel>
              <div className="flex items-center gap-2 mb-3">
                <Clock size={14} strokeWidth={1.5} />
                <h2 className="text-win-section font-semibold">Current Session</h2>
              </div>

              <div className="space-y-1 text-win-small">
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Browser:</span>
                  <span>Chrome on Windows</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-muted-foreground">IP Address:</span>
                  <span className="font-mono">192.168.1.100</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-muted-foreground">Last Activity:</span>
                  <span>Just now</span>
                </div>
              </div>
            </Panel>
          </div>
        </div>
      </div>

      {/* Change Password Dialog */}
      {showPasswordDialog && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="win-border-raised bg-background w-full max-w-[360px]">
            <div className="win-title-bar">
              <div className="flex items-center gap-2">
                <Shield size={14} strokeWidth={1.5} />
                <span>Change Password</span>
              </div>
              <button onClick={closeDialog} className="text-win-small hover:bg-primary/20 px-2">
                X
              </button>
            </div>

            <div className="p-win-md">
              {success ? (
                <div className="flex items-center gap-3 py-4">
                  <Check size={24} className="text-success" strokeWidth={1.5} />
                  <p className="text-win-body">Password changed successfully!</p>
                </div>
              ) : (
                <form onSubmit={handleChangePassword} className="space-y-3">
                  {error && (
                    <div className="win-border-sunken bg-background p-2 flex items-start gap-2">
                      <AlertTriangle
                        size={16}
                        className="text-warning flex-shrink-0 mt-[1px]"
                        strokeWidth={1.5}
                      />
                      <p className="text-win-small text-warning">{error}</p>
                    </div>
                  )}

                  <div>
                    <label htmlFor="currentPassword" className="block text-win-body mb-1">
                      Current Password:
                    </label>
                    <Input
                      id="currentPassword"
                      type="password"
                      value={currentPassword}
                      onChange={(e) => setCurrentPassword(e.target.value)}
                      required
                      disabled={isSubmitting}
                    />
                  </div>

                  <div>
                    <label htmlFor="newPassword" className="block text-win-body mb-1">
                      New Password:
                    </label>
                    <Input
                      id="newPassword"
                      type="password"
                      value={newPassword}
                      onChange={(e) => setNewPassword(e.target.value)}
                      required
                      disabled={isSubmitting}
                      minLength={8}
                    />
                    <p className="text-win-small text-muted-foreground mt-1">
                      Minimum 8 characters
                    </p>
                  </div>

                  <div>
                    <label htmlFor="confirmPassword" className="block text-win-body mb-1">
                      Confirm New Password:
                    </label>
                    <Input
                      id="confirmPassword"
                      type="password"
                      value={confirmPassword}
                      onChange={(e) => setConfirmPassword(e.target.value)}
                      required
                      disabled={isSubmitting}
                    />
                  </div>

                  <div className="flex justify-end gap-2 pt-2 border-t border-border">
                    <Button type="button" onClick={closeDialog} disabled={isSubmitting}>
                      Cancel
                    </Button>
                    <Button
                      type="submit"
                      disabled={isSubmitting || !currentPassword || !newPassword}
                    >
                      {isSubmitting ? 'Changing...' : 'Change Password'}
                    </Button>
                  </div>
                </form>
              )}
            </div>
          </div>
        </div>
      )}
    </AppLayout>
  );
};

export default SettingsPage;
