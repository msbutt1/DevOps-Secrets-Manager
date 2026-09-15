import { fireEvent, render, screen } from '@testing-library/react';
import { ConfirmDialog } from './ConfirmDialog';

describe('ConfirmDialog', () => {
  it('confirms straight away without a typed confirmation', () => {
    const onConfirm = vi.fn();
    render(
      <ConfirmDialog
        isOpen
        title="Delete"
        message="Sure?"
        onConfirm={onConfirm}
        onCancel={vi.fn()}
      />,
    );
    fireEvent.click(screen.getByRole('button', { name: 'OK' }));
    expect(onConfirm).toHaveBeenCalledTimes(1);
  });

  it('requires the exact name before a destructive action', () => {
    const onConfirm = vi.fn();
    render(
      <ConfirmDialog
        isOpen
        title="Delete Vault"
        message="This cannot be undone."
        confirmLabel="Delete Vault"
        confirmText="payments-api"
        confirmTextLabel="vault name"
        onConfirm={onConfirm}
        onCancel={vi.fn()}
      />,
    );
    const confirm = screen.getByRole('button', { name: 'Delete Vault' });
    expect(confirm).toBeDisabled();

    const input = screen.getByLabelText('Type payments-api to confirm');
    fireEvent.change(input, { target: { value: 'payments-ap' } });
    expect(confirm).toBeDisabled();
    fireEvent.change(input, { target: { value: 'PAYMENTS-API' } });
    expect(confirm).toBeDisabled();

    fireEvent.change(input, { target: { value: 'payments-api' } });
    expect(confirm).toBeEnabled();
    fireEvent.click(confirm);
    expect(onConfirm).toHaveBeenCalledTimes(1);
  });

  it('clears what was typed when it reopens', () => {
    const { rerender } = render(
      <ConfirmDialog
        isOpen
        title="Delete"
        message="Sure?"
        confirmText="prod"
        onConfirm={vi.fn()}
        onCancel={vi.fn()}
      />,
    );
    fireEvent.change(screen.getByLabelText('Type prod to confirm'), { target: { value: 'prod' } });
    rerender(
      <ConfirmDialog
        isOpen={false}
        title="Delete"
        message="Sure?"
        confirmText="prod"
        onConfirm={vi.fn()}
        onCancel={vi.fn()}
      />,
    );
    rerender(
      <ConfirmDialog
        isOpen
        title="Delete"
        message="Sure?"
        confirmText="prod"
        onConfirm={vi.fn()}
        onCancel={vi.fn()}
      />,
    );
    expect(screen.getByLabelText('Type prod to confirm')).toHaveValue('');
    expect(screen.getByRole('button', { name: 'OK' })).toBeDisabled();
  });
});
