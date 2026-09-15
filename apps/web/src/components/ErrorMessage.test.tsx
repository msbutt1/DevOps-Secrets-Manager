import { fireEvent, render, screen } from '@testing-library/react';
import { ErrorMessage } from './ErrorMessage';

describe('ErrorMessage', () => {
  it('names the failed action and offers a retry', () => {
    const onRetry = vi.fn();
    render(
      <ErrorMessage
        error={new Error('network down')}
        action="load your vaults"
        onRetry={onRetry}
      />,
    );
    expect(screen.getByText('Could not load your vaults')).toBeInTheDocument();
    expect(screen.getByText('network down')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Try Again' }));
    expect(onRetry).toHaveBeenCalledTimes(1);
  });

  it('falls back for unknown errors and hides retry when it cannot retry', () => {
    render(<ErrorMessage error={'oops'} />);
    expect(screen.getByText('Error')).toBeInTheDocument();
    expect(screen.getByText('An unexpected error occurred')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Try Again' })).not.toBeInTheDocument();
  });
});
