import { Skeleton } from '@/components/ui/skeleton';

interface LoadingStateProps {
  type: 'table' | 'cards' | 'detail' | 'inline' | 'page';
  rows?: number;
  columns?: number;
  message?: string;
}

export const LoadingState = ({ type, rows = 5, columns = 4, message = 'Loading...' }: LoadingStateProps) => {
  if (type === 'inline') {
    return (
      <div className="flex items-center gap-2">
        <div className="h-3 w-3 border-2 border-border border-t-primary animate-spin" />
        <span className="text-win-body text-muted-foreground">{message}</span>
      </div>
    );
  }

  if (type === 'page') {
    return (
      <div className="flex items-center justify-center p-8">
        <div className="flex items-center gap-2">
          <div className="h-4 w-4 border-2 border-border border-t-primary animate-spin" />
          <span className="text-win-body text-muted-foreground">{message}</span>
        </div>
      </div>
    );
  }

  if (type === 'table') {
    return (
      <div className="win-border-sunken bg-input">
        <div className="bg-secondary border-b border-border p-2">
          <div className="flex gap-4">
            {Array.from({ length: columns }).map((_, i) => (
              <Skeleton key={i} className="h-3 flex-1 bg-border" />
            ))}
          </div>
        </div>
        {Array.from({ length: rows }).map((_, rowIdx) => (
          <div 
            key={rowIdx} 
            className={`p-2 border-b border-border/50 ${rowIdx % 2 === 1 ? 'bg-background' : ''}`}
          >
            <div className="flex gap-4">
              {Array.from({ length: columns }).map((_, colIdx) => (
                <Skeleton 
                  key={colIdx} 
                  className="h-3 flex-1 bg-border/50" 
                  style={{ width: `${60 + Math.random() * 40}%` }}
                />
              ))}
            </div>
          </div>
        ))}
      </div>
    );
  }

  if (type === 'cards') {
    return (
      <div className="grid grid-cols-1 gap-2">
        {Array.from({ length: rows }).map((_, i) => (
          <div key={i} className="win-border-groove p-3">
            <Skeleton className="h-4 w-1/3 mb-2 bg-border" />
            <Skeleton className="h-3 w-2/3 mb-1 bg-border/50" />
            <Skeleton className="h-3 w-1/2 bg-border/50" />
          </div>
        ))}
      </div>
    );
  }

  // detail type
  return (
    <div className="space-y-3">
      {Array.from({ length: rows }).map((_, i) => (
        <div key={i} className="win-border-groove p-2">
          <Skeleton className="h-3 w-20 mb-1 bg-border/50" />
          <Skeleton className="h-4 w-32 bg-border" />
        </div>
      ))}
    </div>
  );
};
