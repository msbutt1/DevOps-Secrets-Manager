import { Button, Panel } from '@/components/win95';
import { AlertTriangle } from 'lucide-react';

interface ErrorMessageProps {
  error: Error | unknown;
  /** What failed, e.g. "load the audit log"; shown when the error itself is vague */
  action?: string;
  /** Shows a Try Again button when given */
  onRetry?: () => void;
  isRetrying?: boolean;
}

export const ErrorMessage = ({ error, action, onRetry, isRetrying }: ErrorMessageProps) => {
  const message = error instanceof Error ? error.message : 'An unexpected error occurred';

  return (
    <Panel>
      <div className="flex items-start gap-2 text-warning">
        <AlertTriangle size={16} strokeWidth={1.5} className="flex-shrink-0 mt-[2px]" />
        <div className="flex-1">
          <h2 className="text-win-section font-semibold">
            {action ? `Could not ${action}` : 'Error'}
          </h2>
          <p className="text-win-body text-muted-foreground">{message}</p>
          {onRetry && (
            <div className="mt-2">
              <Button onClick={onRetry} disabled={isRetrying}>
                {isRetrying ? 'Retrying...' : 'Try Again'}
              </Button>
            </div>
          )}
        </div>
      </div>
    </Panel>
  );
};
