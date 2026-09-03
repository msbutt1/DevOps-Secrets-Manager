import { Clock, AlertTriangle } from 'lucide-react';
import { expiryStatus } from '@/lib/expiry';
import { cn } from '@/lib/utils';

interface ExpiryBadgeProps {
  expiresAt?: string | null;
}

/** Shows a secret's expiry date, flagging expired and soon-to-expire values. */
export const ExpiryBadge = ({ expiresAt }: ExpiryBadgeProps) => {
  const status = expiryStatus(expiresAt);
  if (status.state === 'none') return <span className="text-muted-foreground">—</span>;

  const date = new Date(expiresAt!).toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  });
  return (
    <div
      className={cn(
        'flex flex-col',
        status.state === 'expired' && 'text-destructive font-semibold',
        status.state === 'soon' && 'text-warning font-semibold',
      )}
      title={`${status.label} (${new Date(expiresAt!).toLocaleString()})`}
    >
      <span className="flex items-center gap-1">
        {status.state === 'expired' ? (
          <AlertTriangle size={10} strokeWidth={1.5} />
        ) : (
          <Clock size={10} strokeWidth={1.5} />
        )}
        {status.state === 'expired' ? 'EXPIRED' : date}
      </span>
      {status.state !== 'ok' && <span className="text-win-small">{status.label}</span>}
    </div>
  );
};
