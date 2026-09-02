import { useEffect, useState } from 'react';
import { Link, useNavigate, useSearchParams } from 'react-router-dom';
import { Button, Input, Panel } from '@/components/win95';
import { Shield, AlertTriangle, Check } from 'lucide-react';
import { ApiRequestError, authApi } from '@/lib/api-client';
import { passwordProblem } from '@/lib/password';
import { PasswordStrengthHint } from '@/components/PasswordStrengthHint';

export const ResetPasswordPage = () => {
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  // Keep the token in memory and drop it from the address bar and history
  const [token] = useState(() => searchParams.get('token') ?? '');
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [linkInvalid, setLinkInvalid] = useState(false);
  const [done, setDone] = useState(false);

  useEffect(() => {
    if (searchParams.has('token')) navigate('/reset-password', { replace: true });
  }, [searchParams, navigate]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    if (password !== confirmPassword) {
      setError('Passwords do not match');
      return;
    }
    const problem = passwordProblem(password);
    if (problem) {
      setError(problem);
      return;
    }
    setIsSubmitting(true);
    try {
      await authApi.resetPassword({ token, newPassword: password });
      setDone(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not reset the password');
      setLinkInvalid(err instanceof ApiRequestError && err.code === 'invalid_reset_token');
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <div className="min-h-screen bg-[#008080] flex items-center justify-center p-4">
      <div className="win-border-raised bg-background w-full max-w-[400px]">
        <div className="win-title-bar">
          <div className="flex items-center gap-2">
            <Shield size={14} strokeWidth={1.5} />
            <span>Vault Console — Reset Password</span>
          </div>
        </div>

        <div className="p-win-md">
          {done ? (
            <>
              <Panel className="mb-3">
                <div className="flex items-start gap-3">
                  <Check size={24} className="text-success flex-shrink-0" strokeWidth={1.5} />
                  <div>
                    <p className="text-win-body font-semibold mb-1">Password reset</p>
                    <p className="text-win-small text-muted-foreground">
                      All of your sessions were signed out. Log in with your new password.
                    </p>
                  </div>
                </div>
              </Panel>
              <div className="flex justify-end pt-2 border-t border-border">
                <Button onClick={() => navigate('/login')}>Go to Login</Button>
              </div>
            </>
          ) : !token ? (
            <Panel>
              <p className="text-win-body mb-2">This page needs the link from the reset email.</p>
              <Link to="/forgot-password" className="text-win-small text-info hover:underline">
                Request a new link
              </Link>
            </Panel>
          ) : (
            <>
              {error && (
                <div className="win-border-sunken bg-background mb-3 p-2 flex items-start gap-2">
                  <AlertTriangle
                    size={16}
                    className="text-warning flex-shrink-0 mt-[1px]"
                    strokeWidth={1.5}
                  />
                  <div>
                    <p className="text-win-small text-warning">{error}</p>
                    {linkInvalid && (
                      <Link
                        to="/forgot-password"
                        className="text-win-small text-info hover:underline"
                      >
                        Request a new link
                      </Link>
                    )}
                  </div>
                </div>
              )}

              <form onSubmit={handleSubmit} className="space-y-3">
                <div>
                  <label htmlFor="password" className="block text-win-body mb-1">
                    New Password:
                  </label>
                  <Input
                    id="password"
                    type="password"
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    required
                    autoComplete="new-password"
                    disabled={isSubmitting}
                    aria-describedby="password-hint"
                  />
                  <PasswordStrengthHint id="password-hint" password={password} />
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
                    autoComplete="new-password"
                    disabled={isSubmitting}
                  />
                </div>
                <div className="flex justify-end gap-2 pt-2 border-t border-border">
                  <Button type="submit" disabled={isSubmitting || !password || !confirmPassword}>
                    {isSubmitting ? 'Saving...' : 'Reset Password'}
                  </Button>
                </div>
              </form>
            </>
          )}
        </div>
      </div>
    </div>
  );
};

export default ResetPasswordPage;
