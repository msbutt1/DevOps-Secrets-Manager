import { cn } from '@/lib/utils';
import {
  MIN_PASSWORD_LENGTH,
  STRENGTH_LABELS,
  passwordProblem,
  passwordStrength,
} from '@/lib/password';

interface PasswordStrengthHintProps {
  id: string;
  password: string;
  /** The user's name and email; passwords containing them are rejected */
  personal?: string[];
}

const SEGMENT_COLORS = ['bg-destructive', 'bg-destructive', 'bg-warning', 'bg-info', 'bg-success'];

export const PasswordStrengthHint = ({
  id,
  password,
  personal = [],
}: PasswordStrengthHintProps) => {
  if (!password) {
    return (
      <p id={id} className="text-win-small text-muted-foreground mt-1">
        At least {MIN_PASSWORD_LENGTH} characters. A few unrelated words make a strong password.
      </p>
    );
  }

  const problem = passwordProblem(password, personal);
  const strength = passwordStrength(password, personal);

  return (
    <div id={id} className="mt-1" aria-live="polite">
      <div className="flex gap-[2px] win-border-sunken bg-background p-[2px]" aria-hidden="true">
        {[1, 2, 3, 4].map((segment) => (
          <div
            key={segment}
            className={cn('h-[6px] flex-1', segment <= strength && SEGMENT_COLORS[strength])}
          />
        ))}
      </div>
      <p className={cn('text-win-small mt-1', problem ? 'text-warning' : 'text-muted-foreground')}>
        {problem ?? `Strength: ${STRENGTH_LABELS[strength]}`}
      </p>
    </div>
  );
};
