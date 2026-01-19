import { useState, useEffect, useCallback } from 'react';
import { Button, Panel } from '@/components/win95';
import { Eye, Copy, Check, AlertTriangle } from 'lucide-react';

interface RevealSecretDialogProps {
  isOpen: boolean;
  secretName: string;
  secretValue: string | null;
  isLoading?: boolean;
  onClose: () => void;
  onReveal: () => void;
}

export const RevealSecretDialog = ({
  isOpen,
  secretName,
  secretValue,
  isLoading = false,
  onClose,
  onReveal,
}: RevealSecretDialogProps) => {
  const [copied, setCopied] = useState(false);
  const [countdown, setCountdown] = useState(30);
  const [hasRevealed, setHasRevealed] = useState(false);

  // Reset state when dialog opens
  useEffect(() => {
    if (isOpen) {
      setCopied(false);
      setCountdown(30);
      setHasRevealed(false);
    }
  }, [isOpen]);

  // Countdown timer when secret is revealed
  useEffect(() => {
    if (!secretValue || !isOpen) return;
    
    setHasRevealed(true);
    const timer = setInterval(() => {
      setCountdown((prev) => {
        if (prev <= 1) {
          clearInterval(timer);
          onClose();
          return 0;
        }
        return prev - 1;
      });
    }, 1000);

    return () => clearInterval(timer);
  }, [secretValue, isOpen, onClose]);

  const handleCopy = useCallback(async () => {
    if (!secretValue) return;
    
    try {
      await navigator.clipboard.writeText(secretValue);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch (err) {
      console.error('Failed to copy:', err);
    }
  }, [secretValue]);

  const handleRevealClick = () => {
    onReveal();
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      {/* Backdrop */}
      <div className="absolute inset-0 bg-foreground/20" onClick={onClose} />

      {/* Dialog */}
      <div className="relative win-border-raised bg-background w-full max-w-[480px] animate-win-open">
        {/* Title Bar */}
        <div className="win-title-bar">
          <div className="flex items-center gap-2">
            <Eye size={14} />
            <span>Reveal Secret</span>
          </div>
        </div>

        {/* Content */}
        <div className="p-3 space-y-3">
          {/* Warning Panel */}
          <Panel className="flex items-start gap-2 !p-2 border-warning">
            <AlertTriangle size={14} className="text-warning flex-shrink-0 mt-[2px]" strokeWidth={1.5} />
            <div className="text-win-small">
              <strong>Security Notice:</strong> This action is logged for audit purposes. The secret value will be hidden automatically after 30 seconds.
            </div>
          </Panel>

          {/* Secret Name */}
          <div className="win-border-groove p-2">
            <div className="text-win-small text-muted-foreground mb-1">Secret Name</div>
            <div className="font-mono text-win-body font-semibold">{secretName}</div>
          </div>

          {/* Secret Value */}
          {hasRevealed && secretValue ? (
            <>
              <div className="win-border-sunken bg-input p-2">
                <div className="text-win-small text-muted-foreground mb-1">Secret Value</div>
                <div className="font-mono text-win-body break-all bg-info/5 p-2 border border-info/20">
                  {secretValue}
                </div>
              </div>

              {/* Copy Button */}
              <Button
                onClick={handleCopy}
                className="w-full flex items-center justify-center gap-2"
              >
                {copied ? (
                  <>
                    <Check size={12} />
                    Copied to Clipboard
                  </>
                ) : (
                  <>
                    <Copy size={12} />
                    Copy to Clipboard
                  </>
                )}
              </Button>

              {/* Countdown */}
              <div className="text-center text-win-small text-muted-foreground">
                Value will be hidden in <span className="font-semibold text-warning">{countdown}</span> seconds
              </div>
            </>
          ) : (
            <div className="text-center py-4">
              <Button
                onClick={handleRevealClick}
                disabled={isLoading}
                className="flex items-center gap-2 mx-auto"
              >
                <Eye size={12} />
                {isLoading ? 'Decrypting...' : 'Reveal Secret Value'}
              </Button>
              <p className="text-win-small text-muted-foreground mt-2">
                Click to decrypt and display the secret value
              </p>
            </div>
          )}

          {/* Actions */}
          <div className="flex justify-end gap-2 pt-2 border-t border-border">
            <Button onClick={onClose}>Close</Button>
          </div>
        </div>
      </div>
    </div>
  );
};
