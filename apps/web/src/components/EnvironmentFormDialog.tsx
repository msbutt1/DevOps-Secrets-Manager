import { useEffect, useState } from 'react';
import { useDialog } from '@/hooks/use-dialog';
import { Button, Input, Panel } from '@/components/win95';
import { Layers } from 'lucide-react';
import type { EnvironmentCreateRequest } from '@/types/api';
import {
  ENVIRONMENT_NAME_HINT,
  SUGGESTED_ENVIRONMENTS,
  isProductionEnvironment,
  validateEnvironmentName,
} from '@/lib/environments';

interface EnvironmentFormDialogProps {
  isOpen: boolean;
  existingEnvs: string[];
  onClose: () => void;
  onSave: (data: EnvironmentCreateRequest) => void;
}

export const EnvironmentFormDialog = ({
  isOpen,
  existingEnvs,
  onClose,
  onSave,
}: EnvironmentFormDialogProps) => {
  const dialogRef = useDialog<HTMLDivElement>(isOpen, onClose);
  const [name, setName] = useState('');
  const [touched, setTouched] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);

  useEffect(() => {
    if (isOpen) {
      setName('');
      setTouched(false);
    }
  }, [isOpen]);

  const error = validateEnvironmentName(name, existingEnvs);
  const suggestions = SUGGESTED_ENVIRONMENTS.filter((env) => !existingEnvs.includes(env.name));

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setTouched(true);
    if (error) return;

    setIsSubmitting(true);
    try {
      onSave({ name: name.trim() });
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
        className="relative z-10 win-border-raised bg-background w-full max-w-[380px] max-h-[90vh] overflow-auto animate-win-open"
      >
        {/* Title Bar */}
        <div className="win-title-bar">
          <div className="flex items-center gap-2">
            <Layers size={14} />
            <span>Add Environment</span>
          </div>
        </div>

        {/* Content */}
        <form onSubmit={handleSubmit} className="p-3 space-y-3">
          <Panel className="!p-2">
            <p className="text-win-small">
              Name the environment, for example <strong>staging</strong> or{' '}
              <strong>eu-west-1</strong>. Each name can be used once per vault.
            </p>
          </Panel>

          <div>
            <label htmlFor="environment-name" className="block text-win-body mb-1">
              Environment Name: <span className="text-warning">*</span>
            </label>
            <Input
              id="environment-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              onBlur={() => setTouched(true)}
              placeholder="e.g., staging"
              autoFocus
              disabled={isSubmitting}
              aria-invalid={touched && !!error}
              aria-describedby="environment-name-help"
            />
            <p
              id="environment-name-help"
              className={`text-win-small mt-1 ${touched && error ? 'text-warning' : 'text-muted-foreground'}`}
            >
              {touched && error ? error : ENVIRONMENT_NAME_HINT}
            </p>
            {isProductionEnvironment(name.trim()) && (
              <p className="text-win-small text-warning mt-1">
                Production environment: secrets here affect live systems.
              </p>
            )}
          </div>

          {suggestions.length > 0 && (
            <div>
              <div className="text-win-small text-muted-foreground mb-1">Suggestions:</div>
              <div className="flex flex-wrap gap-1">
                {suggestions.map((env) => (
                  <button
                    key={env.name}
                    type="button"
                    title={env.description}
                    onClick={() => {
                      setName(env.name);
                      setTouched(true);
                    }}
                    className={`win-button !min-w-0 !px-2 !py-[2px] text-win-small ${
                      isProductionEnvironment(env.name) ? 'text-warning' : ''
                    }`}
                  >
                    {env.name}
                  </button>
                ))}
              </div>
            </div>
          )}

          {/* Actions */}
          <div className="flex justify-end gap-2 pt-2 border-t border-border">
            <Button type="submit" disabled={isSubmitting || (touched && !!error)}>
              {isSubmitting ? 'Creating...' : 'Create'}
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
