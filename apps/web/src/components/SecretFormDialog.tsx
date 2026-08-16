import { useState, useEffect } from 'react';
import { Button, Input, Panel } from '@/components/win95';
import { AlertTriangle, Plus, X, Key } from 'lucide-react';
import type { Secret, SecretCreateRequest } from '@/types/api';

interface SecretFormDialogProps {
  isOpen: boolean;
  secret?: Secret | null;
  onClose: () => void;
  onSave: (data: SecretCreateRequest) => void;
}

export const SecretFormDialog = ({ isOpen, secret, onClose, onSave }: SecretFormDialogProps) => {
  const [name, setName] = useState('');
  const [value, setValue] = useState('');
  const [description, setDescription] = useState('');
  const [rotationInterval, setRotationInterval] = useState('');
  const [expiresAt, setExpiresAt] = useState('');
  const [labels, setLabels] = useState<{ key: string; value: string }[]>([]);
  const [showValue, setShowValue] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);

  const isEditing = !!secret;

  useEffect(() => {
    if (secret) {
      setName(secret.keyName);
      setValue(''); // Never pre-fill value
      setDescription(secret.description || '');
      setRotationInterval(secret.rotationPolicy?.intervalDays.toString() || '');
      setExpiresAt(secret.expiresAt?.split('T')[0] || '');
      setLabels(Object.entries(secret.metadata || {}).map(([key, value]) => ({ key, value })));
    } else {
      setName('');
      setValue('');
      setDescription('');
      setRotationInterval('');
      setExpiresAt('');
      setLabels([]);
    }
    setShowValue(false);
  }, [secret, isOpen]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsSubmitting(true);

    const labelsObj = labels.reduce(
      (acc, { key, value }) => {
        if (key.trim()) {
          acc[key.trim()] = value;
        }
        return acc;
      },
      {} as Record<string, string>,
    );

    try {
      // Convert date to RFC3339 format for Go backend
      const expiresAtRFC3339 = expiresAt ? `${expiresAt}T00:00:00Z` : undefined;

      onSave({
        keyName: name,
        // Leaving the value blank while editing keeps the stored value
        value: isEditing && !value ? undefined : value,
        description: description || undefined,
        rotationIntervalDays: rotationInterval ? parseInt(rotationInterval) : undefined,
        expiresAt: expiresAtRFC3339,
        metadata: Object.keys(labelsObj).length > 0 ? labelsObj : undefined,
      });
    } finally {
      setIsSubmitting(false);
    }
  };

  const addLabel = () => {
    setLabels([...labels, { key: '', value: '' }]);
  };

  const removeLabel = (index: number) => {
    setLabels(labels.filter((_, i) => i !== index));
  };

  const updateLabel = (index: number, field: 'key' | 'value', newValue: string) => {
    setLabels(labels.map((label, i) => (i === index ? { ...label, [field]: newValue } : label)));
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      {/* Backdrop */}
      <div className="absolute inset-0 bg-foreground/20" onClick={onClose} />

      {/* Dialog */}
      <div className="relative win-border-raised bg-background w-full max-w-[520px] animate-win-open">
        {/* Title Bar */}
        <div className="win-title-bar">
          <div className="flex items-center gap-2">
            <Key size={14} />
            <span>{isEditing ? 'Edit Secret' : 'Create New Secret'}</span>
          </div>
        </div>

        {/* Content */}
        <form onSubmit={handleSubmit} className="p-3 space-y-3 max-h-[80vh] overflow-auto">
          {/* Warning Panel */}
          <Panel className="flex items-start gap-2 !p-2">
            <AlertTriangle
              size={14}
              className="text-warning flex-shrink-0 mt-[2px]"
              strokeWidth={1.5}
            />
            <div className="text-win-small">
              <strong>Security Notice:</strong> Secret values are encrypted at rest using
              AES-256-GCM. Values cannot be recovered without explicit reveal action. All access is
              logged for audit purposes.
            </div>
          </Panel>

          {/* Name Field */}
          <div>
            <label className="block text-win-body mb-1">
              Secret Name: <span className="text-warning">*</span>
            </label>
            <Input
              value={name}
              onChange={(e) => setName(e.target.value.toUpperCase().replace(/[^A-Z0-9_]/g, ''))}
              placeholder="e.g., DATABASE_URL"
              required
              disabled={isEditing || isSubmitting}
            />
            <p className="text-win-small text-muted-foreground mt-1">
              Use UPPER_SNAKE_CASE format. Cannot be changed after creation.
            </p>
          </div>

          {/* Description Field */}
          <div>
            <label className="block text-win-body mb-1">Description:</label>
            <Input
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Human-readable description of this secret"
              disabled={isSubmitting}
              maxLength={200}
            />
          </div>

          {/* Value Field */}
          <div>
            <label className="block text-win-body mb-1">
              Secret Value: <span className="text-warning">*</span>
            </label>
            <div className="relative">
              <textarea
                value={value}
                onChange={(e) => setValue(e.target.value)}
                className="win-input w-full h-[80px] font-mono resize-none pr-16"
                placeholder={isEditing ? '(enter new value to update)' : 'Enter secret value...'}
                required={!isEditing}
                disabled={isSubmitting}
                style={
                  {
                    WebkitTextSecurity: showValue ? 'none' : 'disc',
                  } as React.CSSProperties
                }
              />
              <button
                type="button"
                onClick={() => setShowValue(!showValue)}
                className="absolute right-2 top-2 text-win-small text-info hover:underline"
              >
                {showValue ? 'Hide' : 'Show'}
              </button>
            </div>
            {isEditing && (
              <p className="text-win-small text-muted-foreground mt-1">
                Leave empty to keep the existing value unchanged.
              </p>
            )}
          </div>

          {/* Advanced Options */}
          <div className="border-t border-border pt-3">
            <div className="text-win-body font-semibold mb-2">Advanced Options</div>

            <div className="flex gap-3">
              <div className="flex-1">
                <label className="block text-win-body mb-1">Rotation Interval (days):</label>
                <Input
                  type="number"
                  value={rotationInterval}
                  onChange={(e) => setRotationInterval(e.target.value)}
                  placeholder="e.g., 90"
                  min={1}
                  max={365}
                  disabled={isSubmitting}
                />
                <p className="text-win-small text-muted-foreground mt-1">
                  Alert when secret needs rotation
                </p>
              </div>

              <div className="flex-1">
                <label className="block text-win-body mb-1">Expiration Date:</label>
                <Input
                  type="date"
                  value={expiresAt}
                  onChange={(e) => setExpiresAt(e.target.value)}
                  disabled={isSubmitting}
                  min={new Date().toISOString().split('T')[0]}
                />
                <p className="text-win-small text-muted-foreground mt-1">
                  Secret expires on this date
                </p>
              </div>
            </div>
          </div>

          {/* Labels */}
          <div>
            <div className="flex items-center justify-between mb-1">
              <label className="text-win-body">Labels (key-value metadata):</label>
              <button
                type="button"
                onClick={addLabel}
                className="text-win-small text-info hover:underline flex items-center gap-1"
                disabled={isSubmitting}
              >
                <Plus size={10} /> Add Label
              </button>
            </div>

            {labels.length > 0 ? (
              <div className="win-border-sunken bg-input p-1 space-y-1">
                {labels.map((label, index) => (
                  <div key={index} className="flex gap-1">
                    <Input
                      value={label.key}
                      onChange={(e) => updateLabel(index, 'key', e.target.value)}
                      placeholder="key"
                      className="flex-1"
                      disabled={isSubmitting}
                    />
                    <Input
                      value={label.value}
                      onChange={(e) => updateLabel(index, 'value', e.target.value)}
                      placeholder="value"
                      className="flex-1"
                      disabled={isSubmitting}
                    />
                    <button
                      type="button"
                      onClick={() => removeLabel(index)}
                      className="win-button !min-w-0 !px-1"
                      disabled={isSubmitting}
                    >
                      <X size={12} />
                    </button>
                  </div>
                ))}
              </div>
            ) : (
              <div className="text-win-small text-muted-foreground">
                Labels help organize secrets (e.g., team: backend, tier: critical)
              </div>
            )}
          </div>

          {/* Actions */}
          <div className="flex justify-end gap-2 pt-2 border-t border-border">
            <Button type="submit" disabled={isSubmitting || (!isEditing && (!name || !value))}>
              {isSubmitting ? 'Saving...' : isEditing ? 'Update Secret' : 'Create Secret'}
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
