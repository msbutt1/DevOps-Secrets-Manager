import { useState } from 'react';
import { fireEvent, render, screen } from '@testing-library/react';
import { useDialog } from './use-dialog';

const Dialog = ({ onClose }: { onClose: () => void }) => {
  const ref = useDialog<HTMLDivElement>(true, onClose);
  return (
    <div ref={ref} role="dialog">
      <button>First</button>
      <button>Last</button>
    </div>
  );
};

const Harness = () => {
  const [open, setOpen] = useState(false);
  return (
    <>
      <button onClick={() => setOpen(true)}>Open</button>
      {open && <Dialog onClose={() => setOpen(false)} />}
    </>
  );
};

describe('useDialog', () => {
  it('moves focus into the dialog, keeps Tab inside it and restores focus on close', () => {
    render(<Harness />);
    const opener = screen.getByRole('button', { name: 'Open' });
    opener.focus();
    fireEvent.click(opener);

    const first = screen.getByRole('button', { name: 'First' });
    const last = screen.getByRole('button', { name: 'Last' });
    expect(first).toHaveFocus();

    // Tab from the last element wraps back to the first
    last.focus();
    fireEvent.keyDown(document, { key: 'Tab' });
    expect(first).toHaveFocus();

    // Shift+Tab from the first wraps to the last
    fireEvent.keyDown(document, { key: 'Tab', shiftKey: true });
    expect(last).toHaveFocus();

    fireEvent.keyDown(document, { key: 'Escape' });
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(opener).toHaveFocus();
  });
});
