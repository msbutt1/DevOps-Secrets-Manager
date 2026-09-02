import { useState } from 'react';
import { Link } from 'react-router-dom';
import { Button, Input, Panel } from '@/components/win95';
import { Shield, AlertTriangle, Check } from 'lucide-react';
import { authApi } from '@/lib/api-client';

export const ForgotPasswordPage = () => {
  const [email, setEmail] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [message, setMessage] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setIsSubmitting(true);
    try {
      const result = await authApi.forgotPassword(email);
      setMessage(result.message);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not send the email');
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <div className="min-h-screen bg-[#008080] flex items-center justify-center p-4">
      <div className="win-border-raised bg-background w-full max-w-[380px]">
        <div className="win-title-bar">
          <div className="flex items-center gap-2">
            <Shield size={14} strokeWidth={1.5} />
            <span>Vault Console — Forgot Password</span>
          </div>
        </div>

        <div className="p-win-md">
          {message ? (
            <>
              <Panel className="mb-3">
                <div className="flex items-start gap-3">
                  <Check size={24} className="text-success flex-shrink-0" strokeWidth={1.5} />
                  <div>
                    <p className="text-win-body mb-2" role="status">
                      {message}
                    </p>
                    <p className="text-win-small text-muted-foreground">
                      Check your inbox for a link to choose a new password.
                    </p>
                  </div>
                </div>
              </Panel>
              <div className="flex justify-end pt-2 border-t border-border">
                <Link to="/login" className="text-win-small text-info hover:underline">
                  Back to login
                </Link>
              </div>
            </>
          ) : (
            <>
              <Panel className="mb-3">
                <p className="text-win-body">
                  Enter the email address of your account and we will send you a link to reset your
                  password.
                </p>
              </Panel>

              {error && (
                <div className="win-border-sunken bg-background mb-3 p-2 flex items-start gap-2">
                  <AlertTriangle
                    size={16}
                    className="text-warning flex-shrink-0 mt-[1px]"
                    strokeWidth={1.5}
                  />
                  <p className="text-win-small text-warning">{error}</p>
                </div>
              )}

              <form onSubmit={handleSubmit} className="space-y-3">
                <div>
                  <label htmlFor="email" className="block text-win-body mb-1">
                    Email:
                  </label>
                  <Input
                    id="email"
                    type="email"
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                    required
                    autoComplete="email"
                    disabled={isSubmitting}
                  />
                </div>
                <div className="flex justify-between items-center gap-2 pt-2 border-t border-border">
                  <Link to="/login" className="text-win-small text-info hover:underline">
                    Back to login
                  </Link>
                  <Button type="submit" disabled={isSubmitting || !email}>
                    {isSubmitting ? 'Sending...' : 'Send Reset Link'}
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

export default ForgotPasswordPage;
