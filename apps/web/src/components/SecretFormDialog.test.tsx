import { fireEvent, render, screen } from '@testing-library/react';
import { SecretFormDialog } from './SecretFormDialog';
import type { Secret } from '@/types/api';

const existing: Secret = {
  id: 's1',
  environmentId: 'e1',
  keyName: 'DATABASE_URL',
  description: 'Primary database',
  createdBy: 'u1',
  createdAt: '2026-09-01T00:00:00Z',
  updatedAt: '2026-09-01T00:00:00Z',
  lastUpdatedAt: '2026-09-01T00:00:00Z',
  lastUpdatedById: 'u1',
  lastUpdatedBy: 'Owner',
  rotationPolicy: {
    intervalDays: 90,
    lastRotatedAt: null,
    nextRotationAt: '2026-11-30T00:00:00Z',
  },
  metadata: { team_name: 'payments' },
};

describe('SecretFormDialog', () => {
  it('requires a key name and a value before creating', () => {
    const onSave = vi.fn();
    render(<SecretFormDialog isOpen onClose={vi.fn()} onSave={onSave} />);
    const create = screen.getByRole('button', { name: 'Create Secret' });
    expect(create).toBeDisabled();

    fireEvent.change(screen.getByPlaceholderText('e.g., DATABASE_URL'), {
      target: { value: 'stripe-key 1' },
    });
    // Names are normalised to upper-case letters, digits and underscores.
    expect(screen.getByPlaceholderText('e.g., DATABASE_URL')).toHaveValue('STRIPEKEY1');
    expect(create).toBeDisabled();

    fireEvent.change(screen.getByPlaceholderText('Enter secret value...'), {
      target: { value: 'sk_test_123' },
    });
    expect(create).toBeEnabled();
    fireEvent.click(create);
    expect(onSave).toHaveBeenCalledWith(
      expect.objectContaining({ keyName: 'STRIPEKEY1', value: 'sk_test_123' }),
    );
  });

  it('keeps the stored value when editing without entering a new one', () => {
    const onSave = vi.fn();
    render(<SecretFormDialog isOpen secret={existing} onClose={vi.fn()} onSave={onSave} />);

    expect(screen.getByPlaceholderText('e.g., DATABASE_URL')).toBeDisabled();
    expect(screen.getByPlaceholderText('(enter new value to update)')).toHaveValue('');
    expect(screen.getByPlaceholderText('key')).toHaveValue('team_name');

    fireEvent.change(screen.getByPlaceholderText('Human-readable description of this secret'), {
      target: { value: 'Renamed' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Update Secret' }));

    const saved = onSave.mock.calls[0][0];
    expect(saved.value).toBeUndefined();
    expect(saved.description).toBe('Renamed');
    expect(saved.rotationIntervalDays).toBe(90);
    expect(saved.metadata).toEqual({ team_name: 'payments' });
  });
});
