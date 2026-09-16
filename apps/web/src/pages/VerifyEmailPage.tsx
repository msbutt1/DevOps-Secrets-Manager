import { useState, useEffect } from 'react';
import { useSearchParams, useNavigate } from 'react-router-dom';
import { Button, Panel } from '@/components/win95';
import { Shield, AlertTriangle, Check, Loader2 } from 'lucide-react';
import { authApi } from '@/lib/api-client';
import { useDocumentTitle } from '@/hooks/use-document-title';

export const VerifyEmailPage = () => {
  useDocumentTitle('Verify email');
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const [status, setStatus] = useState<'loading' | 'success' | 'error'>('loading');
  const [error, setError] = useState<string | null>(null);

  const token = searchParams.get('token');

  useEffect(() => {
    if (!token) {
      setStatus('error');
      setError('No verification token provided');
      return;
    }

    const verifyEmail = async () => {
      try {
        await authApi.verifyEmail({ token });
        setStatus('success');
      } catch (err) {
        setStatus('error');
        setError(err instanceof Error ? err.message : 'Verification failed');
      }
    };

    verifyEmail();
  }, [token]);

  return (
    <div className="min-h-screen bg-[#008080] flex items-center justify-center p-4">
      <div className="win-border-raised bg-background w-full max-w-[400px]">
        {/* Title Bar */}
        <div className="win-title-bar">
          <div className="flex items-center gap-2">
            <Shield size={14} strokeWidth={1.5} />
            <span>Vault Console — Email Verification</span>
          </div>
        </div>

        {/* Content */}
        <div className="p-win-md">
          {status === 'loading' && (
            <Panel>
              <div className="flex items-center justify-center gap-3 py-4">
                <Loader2 size={24} className="animate-spin text-info" strokeWidth={1.5} />
                <p className="text-win-body">Verifying your email...</p>
              </div>
            </Panel>
          )}

          {status === 'success' && (
            <>
              <Panel className="mb-3">
                <div className="flex items-start gap-3">
                  <Check size={24} className="text-success flex-shrink-0" strokeWidth={1.5} />
                  <div>
                    <p className="text-win-body font-semibold mb-2">Email Verified!</p>
                    <p className="text-win-body">
                      Your email has been verified successfully. You can now log in to your account.
                    </p>
                  </div>
                </div>
              </Panel>

              <div className="flex justify-end gap-2 pt-2 border-t border-border">
                <Button onClick={() => navigate('/login')}>Go to Login</Button>
              </div>
            </>
          )}

          {status === 'error' && (
            <>
              <Panel className="mb-3">
                <div className="flex items-start gap-3">
                  <AlertTriangle
                    size={24}
                    className="text-warning flex-shrink-0"
                    strokeWidth={1.5}
                  />
                  <div>
                    <p className="text-win-body text-warning font-semibold mb-2">
                      Verification Failed
                    </p>
                    <p className="text-win-body mb-2">{error}</p>
                    <p className="text-win-small text-muted-foreground">
                      The verification link may have expired or already been used. Please try
                      registering again if you haven't verified your email yet.
                    </p>
                  </div>
                </div>
              </Panel>

              <div className="flex justify-end gap-2 pt-2 border-t border-border">
                <Button onClick={() => navigate('/register')}>Register Again</Button>
                <Button onClick={() => navigate('/login')}>Go to Login</Button>
              </div>
            </>
          )}
        </div>

        {/* Status Bar */}
        <div className="win-border-raised bg-background-secondary h-[18px] flex items-center px-2 border-t-0">
          <span className="text-win-small text-muted-foreground">
            {status === 'loading'
              ? 'Processing...'
              : status === 'success'
                ? 'Verification complete'
                : 'Error'}
          </span>
        </div>
      </div>
    </div>
  );
};

export default VerifyEmailPage;
