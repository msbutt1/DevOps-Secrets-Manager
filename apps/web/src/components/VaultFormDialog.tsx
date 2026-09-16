import { useState, useEffect } from 'react';
import { useDialog } from '@/hooks/use-dialog';
import { Button, Input, Panel } from '@/components/win95';
import { Database } from 'lucide-react';
import type { Vault, VaultCreateRequest, VaultUpdateRequest } from '@/types/api';

interface VaultFormDialogProps {
  isOpen: boolean;
  vault?: Vault | null;
  onClose: () => void;
  onSave: (data: VaultCreateRequest | VaultUpdateRequest) => void;
}

export const VaultFormDialog = ({ isOpen, vault, onClose, onSave }: VaultFormDialogProps) => {
  const dialogRef = useDialog<HTMLDivElement>(isOpen, onClose);
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);

  const isEditing = !!vault;

  useEffect(() => {
    if (vault) {
      setName(vault.name);
      setDescription(vault.description || '');
    } else {
      setName('');
      setDescription('');
    }
  }, [vault, isOpen]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsSubmitting(true);

    try {
      if (isEditing) {
        onSave({ name, description } as VaultUpdateRequest);
      } else {
        onSave({ name, description } as VaultCreateRequest);
      }
    } finally {
      setIsSubmitting(false);
    }
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
      {/* Backdrop */}
      <div className="absolute inset-0 bg-foreground/20" onClick={onClose} />

      {/* Dialog */}
      <div
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        className="relative z-10 win-border-raised bg-background w-full max-w-[420px] max-h-[90vh] overflow-auto animate-win-open"
      >
        {/* Title Bar */}
        <div className="win-title-bar">
          <div className="flex items-center gap-2">
            <Database size={14} />
            <span>{isEditing ? 'Edit Vault' : 'Create New Vault'}</span>
          </div>
        </div>

        {/* Content */}
        <form onSubmit={handleSubmit} className="p-3 space-y-3">
          <Panel className="!p-2">
            <p className="text-win-small">
              {isEditing
                ? 'Update vault configuration. Changes will be applied immediately.'
                : 'Create a new vault to organize your secrets. After creation, you can add environments (dev, staging, prod) and secrets.'}
            </p>
          </Panel>

          {/* Name Field */}
          <div>
            <label htmlFor="vault-name" className="block text-win-body mb-1">
              Vault Name: <span className="text-warning">*</span>
            </label>
            <Input
              id="vault-name"
              value={name}
              onChange={(e) => setName(e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, '-'))}
              placeholder="e.g., production-secrets"
              required
              disabled={isSubmitting}
            />
            <p className="text-win-small text-muted-foreground mt-1">
              Lowercase letters, numbers, and hyphens only
            </p>
          </div>

          {/* Description Field */}
          <div>
            <label htmlFor="vault-description" className="block text-win-body mb-1">
              Description:
            </label>
            <textarea
              id="vault-description"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              className="win-input w-full h-[60px] resize-none"
              placeholder="Optional description for this vault..."
              disabled={isSubmitting}
            />
          </div>

          {/* Actions */}
          <div className="flex justify-end gap-2 pt-2 border-t border-border">
            <Button type="submit" disabled={isSubmitting || !name.trim()}>
              {isSubmitting ? 'Saving...' : isEditing ? 'Update' : 'Create'}
            </Button>
            <Button type="button" onClick={onClose} disabled={isSubmitting}>
              Cancel
            </Button>
          </div>
        </form>
      </div>
    </div>
  );
};
