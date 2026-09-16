import { useEffect, useState } from 'react';
import { useDialog } from '@/hooks/use-dialog';
import { AlertTriangle, XCircle, Info } from 'lucide-react';
import { Button, Input } from '@/components/win95';

interface ConfirmDialogProps {
  isOpen: boolean;
  title: string;
  message: string;
  type?: 'info' | 'warning' | 'error';
  confirmLabel?: string;
  cancelLabel?: string;
  /** When set, the exact text the user must type before confirming (e.g. the name being deleted) */
  confirmText?: string;
  /** What the typed text names, shown in the prompt (default "name") */
  confirmTextLabel?: string;
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
  confirmText,
  confirmTextLabel = 'name',
  onConfirm,
  onCancel,
}: ConfirmDialogProps) => {
  const dialogRef = useDialog<HTMLDivElement>(isOpen, onCancel);
  const [typed, setTyped] = useState('');

  // Start empty every time the dialog opens, so a previous answer cannot carry over
  useEffect(() => {
    if (isOpen) setTyped('');
  }, [isOpen]);

  if (!isOpen) return null;

  const { Icon, className } = iconMap[type];
  const canConfirm = !confirmText || typed === confirmText;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      {/* Backdrop */}
      <div className="absolute inset-0 bg-foreground/20" onClick={onCancel} />

      {/* Dialog */}
      <div
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        className="relative z-10 win-border-raised bg-background w-full max-w-[340px] animate-win-open"
      >
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

          {confirmText && (
            <div className="mt-3">
              <label htmlFor="confirm-text" className="block text-win-small mb-1">
                Type the {confirmTextLabel} <strong className="font-mono">{confirmText}</strong> to
                confirm:
              </label>
              <Input
                id="confirm-text"
                value={typed}
                onChange={(e) => setTyped(e.target.value)}
                autoComplete="off"
                spellCheck={false}
                aria-label={`Type ${confirmText} to confirm`}
              />
            </div>
          )}

          <div className="flex justify-end gap-2 mt-4 pt-2 border-t border-border">
            <Button onClick={onConfirm} disabled={!canConfirm}>
              {confirmLabel}
            </Button>
            <Button onClick={onCancel}>{cancelLabel}</Button>
          </div>
        </div>
      </div>
    </div>
  );
};
