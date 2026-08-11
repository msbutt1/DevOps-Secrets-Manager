import { InputHTMLAttributes, forwardRef } from 'react';
import { cn } from '@/lib/utils';

interface CheckboxProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'type'> {
  label?: string;
}

export const Checkbox = forwardRef<HTMLInputElement, CheckboxProps>(
  ({ className, label, id, ...props }, ref) => {
    return (
      <div className="flex items-center gap-2">
        <input
          ref={ref}
          type="checkbox"
          id={id}
          className={cn('win-checkbox', className)}
          {...props}
        />
        {label && (
          <label htmlFor={id} className="text-win-body cursor-pointer select-none">
            {label}
          </label>
        )}
      </div>
    );
  },
);

Checkbox.displayName = 'Checkbox';
