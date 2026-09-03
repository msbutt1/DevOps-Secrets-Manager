import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { SecretHistoryDialog } from './SecretHistoryDialog';
import { secretsApi } from '@/lib/api-client';
import type { Secret } from '@/types/api';

vi.mock('@/lib/api-client', () => ({
  secretsApi: {
    versions: vi.fn(),
    revealVersion: vi.fn(),
    restoreVersion: vi.fn(),
  },
}));

const secret = { id: 's1', keyName: 'DATABASE_URL', version: 2 } as Secret;

function renderDialog(props: { canReveal: boolean; canRestore: boolean }) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <SecretHistoryDialog secret={secret} onClose={vi.fn()} {...props} />
    </QueryClientProvider>,
  );
}

describe('SecretHistoryDialog', () => {
  beforeEach(() => {
    vi.mocked(secretsApi.versions).mockResolvedValue([
      {
        version: 2,
        createdAt: '2026-09-02T00:00:00Z',
        createdById: 'u1',
        createdBy: 'Olive',
        restoredFrom: null,
        current: true,
      },
      {
        version: 1,
        createdAt: '2026-09-01T00:00:00Z',
        createdById: 'u1',
        createdBy: 'Olive',
        restoredFrom: null,
        current: false,
      },
    ]);
  });

  it('reveals and restores earlier versions after confirmation', async () => {
    vi.mocked(secretsApi.revealVersion).mockResolvedValue({
      id: 's1',
      keyName: 'DATABASE_URL',
      value: 'postgres://old',
      expiresIn: 30,
    });
    vi.mocked(secretsApi.restoreVersion).mockResolvedValue({ ...secret, version: 3 });
    renderDialog({ canReveal: true, canRestore: true });

    expect(await screen.findByText('v1')).toBeInTheDocument();
    // The current version cannot be restored
    expect(screen.getAllByRole('button', { name: 'Restore' })).toHaveLength(1);

    fireEvent.click(screen.getAllByRole('button', { name: 'Reveal' })[1]);
    expect(await screen.findByText('postgres://old')).toBeInTheDocument();
    expect(secretsApi.revealVersion).toHaveBeenCalledWith('s1', 1);

    fireEvent.click(screen.getByRole('button', { name: 'Restore' }));
    expect(secretsApi.restoreVersion).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: 'Confirm' }));
    await waitFor(() => expect(secretsApi.restoreVersion).toHaveBeenCalledWith('s1', 1));
    expect(await screen.findByRole('status')).toHaveTextContent('Version 1 restored as version 3');
  });

  it('hides reveal and restore without permission', async () => {
    renderDialog({ canReveal: false, canRestore: false });
    expect(await screen.findByText('v1')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Reveal' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Restore' })).not.toBeInTheDocument();
  });
});
