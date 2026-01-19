import { Panel } from '@/components/win95';
import { AlertTriangle } from 'lucide-react';

interface ErrorMessageProps {
  error: Error | unknown;
}

export const ErrorMessage = ({ error }: ErrorMessageProps) => {
  const message = error instanceof Error ? error.message : 'An unexpected error occurred';

  return (
    <Panel>
      <div className="flex items-center gap-2 text-warning">
        <AlertTriangle size={16} strokeWidth={1.5} />
        <div>
          <h2 className="text-win-section font-semibold">Error</h2>
          <p className="text-win-body text-muted-foreground">{message}</p>
        </div>
      </div>
    </Panel>
  );
};
