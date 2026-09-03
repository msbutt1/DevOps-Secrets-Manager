import { act, fireEvent, render, screen } from '@testing-library/react';
import { RevealSecretDialog } from './RevealSecretDialog';

describe('RevealSecretDialog', () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it('does not show the value until the user asks to reveal it', () => {
    const onReveal = vi.fn();
    render(
      <RevealSecretDialog
        isOpen
        secretName="DATABASE_URL"
        secretValue={null}
        onClose={vi.fn()}
        onReveal={onReveal}
      />,
    );
    expect(screen.getByText('DATABASE_URL')).toBeInTheDocument();
    expect(screen.queryByText('Copy to Clipboard')).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /Reveal Secret Value/ }));
    expect(onReveal).toHaveBeenCalledTimes(1);
  });

  it('shows the value and hides it after the API auto-hide window', () => {
    vi.useFakeTimers();
    const onClose = vi.fn();
    render(
      <RevealSecretDialog
        isOpen
        secretName="DATABASE_URL"
        secretValue="postgres://secret"
        expiresIn={3}
        onClose={onClose}
        onReveal={vi.fn()}
      />,
    );
    expect(screen.getByText('postgres://secret')).toBeInTheDocument();
    expect(screen.getByText(/hidden automatically after 3 seconds/)).toBeInTheDocument();

    act(() => {
      vi.advanceTimersByTime(2000);
    });
    expect(onClose).not.toHaveBeenCalled();
    act(() => {
      vi.advanceTimersByTime(1000);
    });
    expect(onClose).toHaveBeenCalled();
  });

  it('copies the value to the clipboard', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.assign(navigator, { clipboard: { writeText } });
    render(
      <RevealSecretDialog
        isOpen
        secretName="API_KEY"
        secretValue="sk_live_123"
        onClose={vi.fn()}
        onReveal={vi.fn()}
      />,
    );
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: /Copy to Clipboard/ }));
    });
    expect(writeText).toHaveBeenCalledWith('sk_live_123');
    expect(screen.getByText('Copied to Clipboard')).toBeInTheDocument();
  });

  it('renders nothing when closed', () => {
    const { container } = render(
      <RevealSecretDialog
        isOpen={false}
        secretName="X"
        secretValue="hidden"
        onClose={vi.fn()}
        onReveal={vi.fn()}
      />,
    );
    expect(container).toBeEmptyDOMElement();
  });

  it('warns when the secret has expired but still allows revealing it', () => {
    render(
      <RevealSecretDialog
        isOpen
        secretName="OLD_KEY"
        secretValue={null}
        expiresAt="2020-01-01T00:00:00Z"
        onClose={vi.fn()}
        onReveal={vi.fn()}
      />,
    );
    expect(screen.getByRole('alert')).toHaveTextContent('this value expired');
    expect(screen.getByRole('button', { name: /Reveal Secret Value/ })).toBeEnabled();
  });
});
