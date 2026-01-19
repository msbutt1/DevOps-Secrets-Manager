import { useState, useCallback } from 'react';
import { Copy, Check } from 'lucide-react';
import { Button } from '@/components/win95';

interface CopyButtonProps {
  value: string;
  label?: string;
  size?: 'sm' | 'md';
  variant?: 'button' | 'icon';
  onCopy?: () => void;
}

export const CopyButton = ({ 
  value, 
  label = 'Copy', 
  size = 'sm',
  variant = 'icon',
  onCopy,
}: CopyButtonProps) => {
  const [copied, setCopied] = useState(false);

  const handleCopy = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(true);
      onCopy?.();
      setTimeout(() => setCopied(false), 2000);
    } catch (err) {
      console.error('Failed to copy:', err);
    }
  }, [value, onCopy]);

  if (variant === 'button') {
    return (
      <Button 
        onClick={handleCopy}
        className={`flex items-center gap-1 ${size === 'sm' ? '!min-w-0 !px-2 !py-1' : ''}`}
      >
        {copied ? <Check size={12} /> : <Copy size={12} />}
        {copied ? 'Copied' : label}
      </Button>
    );
  }

  return (
    <button
      onClick={handleCopy}
      className="win-button !min-w-0 !px-1 !py-[2px]"
      title={copied ? 'Copied!' : label}
    >
      {copied ? <Check size={12} className="text-success" /> : <Copy size={12} />}
    </button>
  );
};
