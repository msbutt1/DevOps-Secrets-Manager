import { useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { Button, Input, Panel } from '@/components/win95';
import { Shield, AlertTriangle, Check } from 'lucide-react';
import { authApi } from '@/lib/api-client';

export const RegisterPage = () => {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [name, setName] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);
  const navigate = useNavigate();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    if (password !== confirmPassword) {
      setError('Passwords do not match');
      return;
    }

    if (password.length < 8) {
      setError('Password must be at least 8 characters');
      return;
    }

    setIsSubmitting(true);

    try {
      await authApi.register({ email, password, name });
      setSuccess(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Registration failed');
    } finally {
      setIsSubmitting(false);
    }
  };

  if (success) {
    return (
      <div className="min-h-screen bg-[#008080] flex items-center justify-center p-4">
        <div className="win-border-raised bg-background w-full max-w-[400px]">
          <div className="win-title-bar">
            <div className="flex items-center gap-2">
              <Shield size={14} strokeWidth={1.5} />
              <span>Vault Console — Registration Complete</span>
            </div>
          </div>

          <div className="p-win-md">
            <Panel className="mb-3">
              <div className="flex items-start gap-3">
                <Check size={24} className="text-success flex-shrink-0" strokeWidth={1.5} />
                <div>
                  <p className="text-win-body font-semibold mb-2">Registration Successful!</p>
                  <p className="text-win-body mb-2">
                    We've sent a verification email to <strong>{email}</strong>.
                  </p>
                  <p className="text-win-small text-muted-foreground">
                    Please check your inbox and click the verification link to activate your
                    account. The link expires in 24 hours.
                  </p>
                </div>
              </div>
            </Panel>

            <div className="flex justify-end gap-2 pt-2 border-t border-border">
              <Button onClick={() => navigate('/login')}>Go to Login</Button>
            </div>
          </div>

          <div className="win-border-raised bg-background-secondary h-[18px] flex items-center px-2 border-t-0">
            <span className="text-win-small text-muted-foreground">
              Check your email to continue
            </span>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-[#008080] flex items-center justify-center p-4">
      <div className="win-border-raised bg-background w-full max-w-[400px]">
        {/* Title Bar */}
        <div className="win-title-bar">
          <div className="flex items-center gap-2">
            <Shield size={14} strokeWidth={1.5} />
            <span>Vault Console — Register</span>
          </div>
        </div>

        {/* Content */}
        <div className="p-win-md">
          <Panel className="mb-3">
            <p className="text-win-body mb-2">
              Create an account to access the secrets management console.
            </p>
            <p className="text-win-small text-muted-foreground">Email verification is required.</p>
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
                <p className="text-win-body text-warning font-semibold">Registration Failed</p>
                <p className="text-win-small">{error}</p>
              </div>
            </div>
          )}

          {/* Register Form */}
          <form onSubmit={handleSubmit} className="space-y-3">
            <div>
              <label htmlFor="name" className="block text-win-body mb-1">
                Full Name:
              </label>
              <Input
                id="name"
                type="text"
                value={name}
                onChange={(e) => setName(e.target.value)}
                required
                autoComplete="name"
                disabled={isSubmitting}
              />
            </div>

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
                autoComplete="new-password"
                disabled={isSubmitting}
                minLength={8}
              />
              <p className="text-win-small text-muted-foreground mt-1">Minimum 8 characters</p>
            </div>

            <div>
              <label htmlFor="confirmPassword" className="block text-win-body mb-1">
                Confirm Password:
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

            <div className="flex justify-between items-center gap-2 pt-2 border-t border-border">
              <Link to="/login" className="text-win-small text-info hover:underline">
                Already have an account?
              </Link>
              <Button type="submit" disabled={isSubmitting || !email || !password || !name}>
                {isSubmitting ? 'Creating Account...' : 'Register'}
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

export default RegisterPage;
