import { useState } from 'react';
import { Button } from '@/components/win95';
import { authApi } from '@/lib/api-client';

interface ResendVerificationButtonProps {
  email: string;
}

/** Asks the API to email a new verification link and shows its answer. */
export const ResendVerificationButton = ({ email }: ResendVerificationButtonProps) => {
  const [status, setStatus] = useState<'idle' | 'sending' | 'sent'>('idle');
  const [message, setMessage] = useState<string | null>(null);

  const resend = async () => {
    setStatus('sending');
    setMessage(null);
    try {
      const result = await authApi.resendVerification(email);
      setMessage(result.message);
      setStatus('sent');
    } catch (err) {
      setMessage(err instanceof Error ? err.message : 'Could not send the email');
      setStatus('idle');
    }
  };

  return (
    <div className="space-y-1">
      <Button type="button" onClick={resend} disabled={!email || status !== 'idle'}>
        {status === 'sending' ? 'Sending...' : 'Resend Verification Email'}
      </Button>
      {message && (
        <p className="text-win-small text-muted-foreground" role="status">
          {message}
        </p>
      )}
    </div>
  );
};
