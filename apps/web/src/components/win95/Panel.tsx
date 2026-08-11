import { ReactNode } from 'react';
import { cn } from '@/lib/utils';

interface PanelProps {
  title?: string;
  children: ReactNode;
  className?: string;
}

export const Panel = ({ title, children, className }: PanelProps) => {
  return (
    <div className={cn('win-panel', className)}>
      {title && (
        <div className="text-win-body font-semibold mb-2 -mt-4 -ml-1 bg-background px-1 w-fit">
          {title}
        </div>
      )}
      {children}
    </div>
  );
};
