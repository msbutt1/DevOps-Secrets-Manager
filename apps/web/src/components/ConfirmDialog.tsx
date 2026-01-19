import { AlertTriangle, XCircle, Info } from 'lucide-react';
import { Button } from '@/components/win95';

interface ConfirmDialogProps {
  isOpen: boolean;
  title: string;
  message: string;
  type?: 'info' | 'warning' | 'error';
  confirmLabel?: string;
  cancelLabel?: string;
  onConfirm: () => void;
  onCancel: () => void;
}

const iconMap = {
  info: { Icon: Info, className: 'text-info' },
  warning: { Icon: AlertTriangle, className: 'text-warning' },
  error: { Icon: XCircle, className: 'text-warning' },
};

export const ConfirmDialog = ({
  isOpen,
  title,
  message,
  type = 'warning',
  confirmLabel = 'OK',
  cancelLabel = 'Cancel',
  onConfirm,
  onCancel,
}: ConfirmDialogProps) => {
  if (!isOpen) return null;

  const { Icon, className } = iconMap[type];

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      {/* Backdrop */}
      <div className="absolute inset-0 bg-foreground/20" onClick={onCancel} />

      {/* Dialog */}
      <div className="relative win-border-raised bg-background w-full max-w-[340px] animate-win-open">
        {/* Title Bar */}
        <div className="win-title-bar">
          <span>{title}</span>
        </div>

        {/* Content */}
        <div className="p-3">
          <div className="flex gap-3 items-start">
            <Icon size={32} className={className} strokeWidth={1.5} />
            <p className="text-win-body flex-1 pt-1">{message}</p>
          </div>

          <div className="flex justify-end gap-2 mt-4 pt-2 border-t border-border">
            <Button onClick={onConfirm}>{confirmLabel}</Button>
            <Button onClick={onCancel}>{cancelLabel}</Button>
          </div>
        </div>
      </div>
    </div>
  );
};
