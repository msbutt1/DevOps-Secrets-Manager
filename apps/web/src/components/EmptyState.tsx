import { ReactNode } from 'react';
import { Button } from '@/components/win95';
import { Inbox, Database, Key, Users, FileText } from 'lucide-react';

interface EmptyStateProps {
  type: 'vaults' | 'secrets' | 'members' | 'audit' | 'generic';
  title?: string;
  message?: string;
  actionLabel?: string;
  onAction?: () => void;
  children?: ReactNode;
}

const defaultContent: Record<string, { title: string; message: string; Icon: typeof Inbox }> = {
  vaults: {
    title: 'No Vaults Found',
    message: 'You don\'t have access to any vaults yet. Create a new vault or request access from an administrator.',
    Icon: Database,
  },
  secrets: {
    title: 'No Secrets',
    message: 'This environment doesn\'t contain any secrets yet. Add your first secret to get started.',
    Icon: Key,
  },
  members: {
    title: 'No Members',
    message: 'No members have been added to this vault yet.',
    Icon: Users,
  },
  audit: {
    title: 'No Audit Events',
    message: 'No audit events match the selected filters.',
    Icon: FileText,
  },
  generic: {
    title: 'No Results',
    message: 'No data available.',
    Icon: Inbox,
  },
};

export const EmptyState = ({
  type,
  title,
  message,
  actionLabel,
  onAction,
  children,
}: EmptyStateProps) => {
  const content = defaultContent[type] || defaultContent.generic;
  const Icon = content.Icon;

  return (
    <div className="win-border-sunken bg-input p-8 text-center">
      <Icon 
        size={32} 
        className="mx-auto text-muted-foreground mb-2" 
        strokeWidth={1} 
      />
      <h3 className="text-win-section font-semibold mb-1">
        {title || content.title}
      </h3>
      <p className="text-win-body text-muted-foreground mb-4 max-w-md mx-auto">
        {message || content.message}
      </p>
      {actionLabel && onAction && (
        <Button onClick={onAction}>{actionLabel}</Button>
      )}
      {children}
    </div>
  );
};
