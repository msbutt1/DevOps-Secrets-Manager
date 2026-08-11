import { useState } from 'react';
import { Button, Panel } from '@/components/win95';
import { Layers, AlertTriangle } from 'lucide-react';
import type { EnvironmentName, EnvironmentCreateRequest } from '@/types/api';

interface EnvironmentFormDialogProps {
  isOpen: boolean;
  existingEnvs: EnvironmentName[];
  onClose: () => void;
  onSave: (data: EnvironmentCreateRequest) => void;
}

const ENV_OPTIONS: { value: EnvironmentName; label: string; description: string }[] = [
  { value: 'dev', label: 'Development', description: 'For local development and testing' },
  { value: 'staging', label: 'Staging', description: 'Pre-production testing environment' },
  { value: 'prod', label: 'Production', description: 'Live production environment' },
];

export const EnvironmentFormDialog = ({
  isOpen,
  existingEnvs,
  onClose,
  onSave,
}: EnvironmentFormDialogProps) => {
  const [selectedEnv, setSelectedEnv] = useState<EnvironmentName | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  const availableEnvs = ENV_OPTIONS.filter((env) => !existingEnvs.includes(env.value));

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedEnv) return;

    setIsSubmitting(true);
    try {
      onSave({ name: selectedEnv });
    } finally {
      setIsSubmitting(false);
      setSelectedEnv(null);
    }
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      {/* Backdrop */}
      <div className="absolute inset-0 bg-foreground/20" onClick={onClose} />

      {/* Dialog */}
      <div className="relative win-border-raised bg-background w-full max-w-[380px] animate-win-open">
        {/* Title Bar */}
        <div className="win-title-bar">
          <div className="flex items-center gap-2">
            <Layers size={14} />
            <span>Add Environment</span>
          </div>
        </div>

        {/* Content */}
        <form onSubmit={handleSubmit} className="p-3 space-y-3">
          {availableEnvs.length === 0 ? (
            <Panel className="flex items-start gap-2 !p-2">
              <AlertTriangle
                size={14}
                className="text-warning flex-shrink-0 mt-[2px]"
                strokeWidth={1.5}
              />
              <div className="text-win-body">
                All environment types have been created for this vault.
              </div>
            </Panel>
          ) : (
            <>
              <Panel className="!p-2">
                <p className="text-win-small">
                  Select an environment type to add to this vault. Each vault can have one of each
                  environment type.
                </p>
              </Panel>

              <div className="space-y-2">
                {availableEnvs.map((env) => (
                  <label
                    key={env.value}
                    className={`block win-border-groove p-2 cursor-pointer ${
                      selectedEnv === env.value ? 'bg-primary/10 border-primary' : ''
                    } ${env.value === 'prod' ? 'border-warning/50' : ''}`}
                  >
                    <div className="flex items-start gap-2">
                      <input
                        type="radio"
                        name="environment"
                        value={env.value}
                        checked={selectedEnv === env.value}
                        onChange={() => setSelectedEnv(env.value)}
                        className="win-checkbox mt-1"
                      />
                      <div>
                        <div
                          className={`text-win-body font-semibold ${
                            env.value === 'prod' ? 'text-warning' : ''
                          }`}
                        >
                          {env.label}
                          {env.value === 'prod' && (
                            <span className="ml-2 text-win-small">(CRITICAL)</span>
                          )}
                        </div>
                        <div className="text-win-small text-muted-foreground">
                          {env.description}
                        </div>
                      </div>
                    </div>
                  </label>
                ))}
              </div>
            </>
          )}

          {/* Actions */}
          <div className="flex justify-end gap-2 pt-2 border-t border-border">
            {availableEnvs.length > 0 && (
              <Button type="submit" disabled={isSubmitting || !selectedEnv}>
                {isSubmitting ? 'Creating...' : 'Create'}
              </Button>
            )}
            <Button type="button" onClick={onClose} disabled={isSubmitting}>
              {availableEnvs.length === 0 ? 'Close' : 'Cancel'}
            </Button>
          </div>
        </form>
      </div>
    </div>
  );
};
