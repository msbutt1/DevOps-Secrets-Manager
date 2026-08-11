import { useState } from 'react';
import { Link, useNavigate, useLocation } from 'react-router-dom';
import { useAuth } from '@/contexts/AuthContext';
import { Button, Input, Panel } from '@/components/win95';
import { Shield, AlertTriangle } from 'lucide-react';

export const LoginPage = () => {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const { login, error, clearError } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();

  const from = (location.state as { from?: { pathname: string } })?.from?.pathname || '/';

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    clearError();
    setIsSubmitting(true);

    try {
      await login({ email, password });
      navigate(from, { replace: true });
    } catch {
      // Error is handled by AuthContext
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <div className="min-h-screen bg-[#008080] flex items-center justify-center p-4">
      <div className="win-border-raised bg-background w-full max-w-[360px]">
        {/* Title Bar */}
        <div className="win-title-bar">
          <div className="flex items-center gap-2">
            <Shield size={14} strokeWidth={1.5} />
            <span>Vault Console — Login</span>
          </div>
        </div>

        {/* Content */}
        <div className="p-win-md">
          <Panel className="mb-3">
            <p className="text-win-body mb-2">
              Enter your credentials to access the secrets management console.
            </p>
            <p className="text-win-small text-muted-foreground">All access attempts are logged.</p>
          </Panel>

          {/* Error Display */}
          {error && (
            <div className="win-border-sunken bg-background mb-3 p-2 flex items-start gap-2">
              <AlertTriangle
                size={16}
                className="text-warning flex-shrink-0 mt-[1px]"
                strokeWidth={1.5}
              />
              <div>
                <p className="text-win-body text-warning font-semibold">Authentication Failed</p>
                <p className="text-win-small">{error}</p>
              </div>
            </div>
          )}

          {/* Login Form */}
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

            <div>
              <label htmlFor="password" className="block text-win-body mb-1">
                Password:
              </label>
              <Input
                id="password"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
                autoComplete="current-password"
                disabled={isSubmitting}
              />
            </div>

            <div className="flex justify-between items-center gap-2 pt-2 border-t border-border">
              <Link to="/register" className="text-win-small text-info hover:underline">
                Create an account
              </Link>
              <Button type="submit" disabled={isSubmitting || !email || !password}>
                {isSubmitting ? 'Authenticating...' : 'Login'}
              </Button>
            </div>
          </form>
        </div>

        {/* Status Bar */}
        <div className="win-border-raised bg-background-secondary h-[18px] flex items-center px-2 border-t-0">
          <span className="text-win-small text-muted-foreground">
            Secure connection established
          </span>
        </div>
      </div>
    </div>
  );
};

export default LoginPage;
