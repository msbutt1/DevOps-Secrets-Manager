import { useState, useCallback, useMemo } from 'react';
import { AppLayout } from '@/components/AppLayout';
import { Panel, Button, Input, Select } from '@/components/win95';
import { EmptyState } from '@/components/EmptyState';
import { ChevronLeft, ChevronRight, X, Download, FileText } from 'lucide-react';
import type { AuditEvent, AuditAction } from '@/types/api';
import { useAuditLogs } from '@/hooks/use-audit';
import { useVaults } from '@/hooks/use-vaults';

const actionOptions: { value: string; label: string }[] = [
  { value: '', label: 'All Actions' },
  { value: 'login.success', label: 'Login Success' },
  { value: 'login.failure', label: 'Login Failure' },
  { value: 'secret.created', label: 'Secret Created' },
  { value: 'secret.updated', label: 'Secret Updated' },
  { value: 'secret.deleted', label: 'Secret Deleted' },
  { value: 'secret.revealed', label: 'Secret Revealed' },
  { value: 'member.added', label: 'Member Added' },
  { value: 'member.removed', label: 'Member Removed' },
  { value: 'member.role_changed', label: 'Role Changed' },
  { value: 'vault.created', label: 'Vault Created' },
  { value: 'vault.deleted', label: 'Vault Deleted' },
];

const formatTimestamp = (ts: string) => {
  const date = new Date(ts);
  return date.toLocaleString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  });
};

const actionColors: Record<string, string> = {
  'secret.revealed': 'text-warning',
  'secret.deleted': 'text-warning',
  'member.removed': 'text-warning',
  'login.failure': 'text-warning',
  'vault.deleted': 'text-warning',
  'secret.created': 'text-success',
  'member.added': 'text-success',
  'vault.created': 'text-success',
  'login.success': 'text-info',
};

const actionIcons: Record<string, string> = {
  'secret.revealed': 'REVEAL',
  'secret.deleted': 'DELETE',
  'secret.created': 'CREATE',
  'secret.updated': 'UPDATE',
  'member.added': 'ADD',
  'member.removed': 'REMOVE',
  'vault.created': 'CREATE',
  'vault.deleted': 'DELETE',
  'login.success': 'LOGIN',
  'login.failure': 'FAIL',
};

export const AuditPage = () => {
  const [selectedEvent, setSelectedEvent] = useState<AuditEvent | null>(null);
  const [currentPage, setCurrentPage] = useState(1);
  const [filters, setFilters] = useState({
    vault: '',
    action: '',
    user: '',
    startDate: '',
    endDate: '',
  });

  const pageSize = 25;

  // Fetch vaults for filter dropdown
  const { data: vaults } = useVaults();

  // Build API filters from UI state
  const apiFilters = {
    vaultId: filters.vault || undefined,
    action: (filters.action as AuditAction) || undefined,
    userId: filters.user || undefined,
    startDate: filters.startDate || undefined,
    endDate: filters.endDate || undefined,
    page: currentPage,
    limit: pageSize,
  };

  // Fetch audit logs
  const { data, isLoading, error } = useAuditLogs(apiFilters);

  const events = useMemo(() => data?.data ?? [], [data]);
  const total = data?.total || 0;
  const totalPages = Math.ceil(total / pageSize);

  const clearFilters = () => {
    setFilters({
      vault: '',
      action: '',
      user: '',
      startDate: '',
      endDate: '',
    });
    setCurrentPage(1);
  };

  const hasActiveFilters = Object.values(filters).some((v) => v !== '');

  const exportToCSV = useCallback(() => {
    const headers = ['Timestamp', 'Action', 'User', 'Vault', 'Environment', 'Target', 'IP Address'];
    const rows = events.map((e) => [
      formatTimestamp(e.timestamp),
      e.action,
      e.userEmail,
      e.vaultName,
      e.environmentName || '',
      e.targetName || '',
      e.ipAddress,
    ]);

    const csvContent = [
      headers.join(','),
      ...rows.map((row) => row.map((cell) => `"${cell}"`).join(',')),
    ].join('\n');

    const blob = new Blob([csvContent], { type: 'text/csv' });
    const url = window.URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `audit-log-${new Date().toISOString().split('T')[0]}.csv`;
    a.click();
    window.URL.revokeObjectURL(url);
  }, [events]);

  // Build vault options from fetched data
  const vaultOptions: { value: string; label: string }[] = [
    { value: '', label: 'All Vaults' },
    ...(vaults?.map((v) => ({ value: v.id, label: v.name })) || []),
  ];

  return (
    <AppLayout>
      <div className="space-y-win-sm h-full flex flex-col">
        {/* Page Header */}
        <div className="win-border-raised bg-background p-2 flex items-center justify-between">
          <div>
            <h1 className="text-win-title font-semibold">Audit Log</h1>
            <p className="text-win-body text-muted-foreground">
              Immutable record of all system actions
            </p>
          </div>
          <Button
            onClick={exportToCSV}
            className="flex items-center gap-1"
            disabled={events.length === 0 || isLoading}
          >
            <Download size={12} strokeWidth={1.5} />
            Export CSV
          </Button>
        </div>

        <div className="flex gap-win-sm flex-1 min-h-0">
          {/* Main Panel */}
          <div className="flex-1 flex flex-col">
            {/* Filters */}
            <Panel className="mb-win-sm">
              <div className="flex items-end gap-2 flex-wrap">
                <div>
                  <label className="block text-win-body mb-1">Vault:</label>
                  <Select
                    value={filters.vault}
                    onChange={(e) => setFilters({ ...filters, vault: e.target.value })}
                    options={vaultOptions}
                    className="w-[160px]"
                  />
                </div>
                <div>
                  <label className="block text-win-body mb-1">Action:</label>
                  <Select
                    value={filters.action}
                    onChange={(e) => setFilters({ ...filters, action: e.target.value })}
                    options={actionOptions}
                    className="w-[160px]"
                  />
                </div>
                <div>
                  <label className="block text-win-body mb-1">User:</label>
                  <Input
                    value={filters.user}
                    onChange={(e) => setFilters({ ...filters, user: e.target.value })}
                    placeholder="Search user..."
                    className="w-[160px]"
                  />
                </div>
                <div>
                  <label className="block text-win-body mb-1">From:</label>
                  <Input
                    type="date"
                    value={filters.startDate}
                    onChange={(e) => setFilters({ ...filters, startDate: e.target.value })}
                    className="w-[130px]"
                  />
                </div>
                <div>
                  <label className="block text-win-body mb-1">To:</label>
                  <Input
                    type="date"
                    value={filters.endDate}
                    onChange={(e) => setFilters({ ...filters, endDate: e.target.value })}
                    className="w-[130px]"
                  />
                </div>
                {hasActiveFilters && (
                  <Button onClick={clearFilters} className="flex items-center gap-1 self-end">
                    <X size={12} />
                    Clear
                  </Button>
                )}
              </div>
            </Panel>

            {/* Events Table */}
            {error ? (
              <Panel className="flex-1 flex items-center justify-center">
                <div className="text-center">
                  <p className="text-warning font-semibold mb-2">Error loading audit logs</p>
                  <p className="text-win-body text-muted-foreground">{error.message}</p>
                </div>
              </Panel>
            ) : isLoading ? (
              <Panel className="flex-1 flex items-center justify-center">
                <div className="text-center">
                  <p className="text-win-body text-muted-foreground">Loading audit logs...</p>
                </div>
              </Panel>
            ) : events.length === 0 ? (
              <EmptyState type="audit" />
            ) : (
              <div className="flex-1 win-border-sunken bg-input overflow-auto">
                <table className="w-full text-win-body">
                  <thead className="sticky top-0 z-10">
                    <tr className="bg-secondary border-b border-border">
                      <th className="text-left px-2 py-1 font-semibold w-[160px]">Timestamp</th>
                      <th className="text-left px-2 py-1 font-semibold w-[120px]">Action</th>
                      <th className="text-left px-2 py-1 font-semibold">Target</th>
                      <th className="text-left px-2 py-1 font-semibold w-[160px]">User</th>
                      <th className="text-left px-2 py-1 font-semibold w-[120px]">Vault</th>
                      <th className="text-left px-2 py-1 font-semibold w-[80px]">Env</th>
                    </tr>
                  </thead>
                  <tbody>
                    {events.map((event, idx) => (
                      <tr
                        key={event.id}
                        onClick={() => setSelectedEvent(event)}
                        className={`border-b border-border/50 cursor-pointer ${
                          selectedEvent?.id === event.id
                            ? 'bg-primary text-primary-foreground'
                            : idx % 2 === 1
                              ? 'bg-background hover:bg-primary/10'
                              : 'hover:bg-primary/10'
                        }`}
                      >
                        <td className="px-2 py-1 text-win-small font-mono">
                          {formatTimestamp(event.timestamp)}
                        </td>
                        <td
                          className={`px-2 py-1 ${
                            selectedEvent?.id === event.id ? '' : actionColors[event.action] || ''
                          }`}
                        >
                          <span className="font-semibold text-win-small">
                            {actionIcons[event.action] || 'ACTION'}
                          </span>
                          <div className="text-win-small opacity-80">
                            {event.action.split('.')[0]}
                          </div>
                        </td>
                        <td className="px-2 py-1 font-mono">{event.targetName || '—'}</td>
                        <td className="px-2 py-1">{event.userEmail}</td>
                        <td className="px-2 py-1">{event.vaultName}</td>
                        <td
                          className={`px-2 py-1 ${
                            event.environmentName === 'prod' && selectedEvent?.id !== event.id
                              ? 'text-warning font-semibold'
                              : ''
                          }`}
                        >
                          {event.environmentName || '—'}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}

            {/* Pagination */}
            <div className="win-border-raised bg-background p-1 flex items-center justify-between mt-1">
              <span className="text-win-small text-muted-foreground">
                Showing {total > 0 ? (currentPage - 1) * pageSize + 1 : 0}-
                {Math.min(currentPage * pageSize, total)} of {total} events
              </span>
              <div className="flex gap-1 items-center">
                <Button
                  className="!min-w-0 !px-2"
                  disabled={currentPage === 1 || isLoading}
                  onClick={() => setCurrentPage((p) => p - 1)}
                >
                  <ChevronLeft size={12} />
                </Button>
                <span className="text-win-body px-2">
                  Page {currentPage} of {totalPages || 1}
                </span>
                <Button
                  className="!min-w-0 !px-2"
                  disabled={currentPage >= totalPages || isLoading}
                  onClick={() => setCurrentPage((p) => p + 1)}
                >
                  <ChevronRight size={12} />
                </Button>
              </div>
            </div>
          </div>

          {/* Detail Panel */}
          <div className="w-[320px]">
            <Panel className="h-full">
              <div className="flex items-center gap-2 mb-2">
                <FileText size={14} strokeWidth={1.5} />
                <h2 className="text-win-section font-semibold">Event Details</h2>
              </div>

              {selectedEvent ? (
                <div className="space-y-2 text-win-body">
                  <div className="win-border-groove p-2">
                    <div className="text-win-small text-muted-foreground mb-1">Event ID</div>
                    <div className="font-mono text-win-small">{selectedEvent.id}</div>
                  </div>

                  <div className="win-border-groove p-2">
                    <div className="text-win-small text-muted-foreground mb-1">Timestamp</div>
                    <div>{formatTimestamp(selectedEvent.timestamp)}</div>
                  </div>

                  <div className="win-border-groove p-2">
                    <div className="text-win-small text-muted-foreground mb-1">Action</div>
                    <div className={`font-semibold ${actionColors[selectedEvent.action] || ''}`}>
                      {selectedEvent.action}
                    </div>
                  </div>

                  <div className="win-border-groove p-2">
                    <div className="text-win-small text-muted-foreground mb-1">User</div>
                    <div>{selectedEvent.userEmail}</div>
                    <div className="text-win-small text-muted-foreground font-mono">
                      {selectedEvent.userId}
                    </div>
                  </div>

                  <div className="win-border-groove p-2">
                    <div className="text-win-small text-muted-foreground mb-1">Target</div>
                    <div className="font-mono">{selectedEvent.targetName || 'N/A'}</div>
                    {selectedEvent.targetId && (
                      <div className="text-win-small text-muted-foreground font-mono">
                        {selectedEvent.targetId}
                      </div>
                    )}
                  </div>

                  <div className="win-border-groove p-2">
                    <div className="text-win-small text-muted-foreground mb-1">Location</div>
                    <div>
                      {selectedEvent.vaultName}
                      {selectedEvent.environmentName && (
                        <span
                          className={selectedEvent.environmentName === 'prod' ? 'text-warning' : ''}
                        >
                          {' / '}
                          {selectedEvent.environmentName}
                        </span>
                      )}
                    </div>
                  </div>

                  <div className="win-border-groove p-2">
                    <div className="text-win-small text-muted-foreground mb-1">IP Address</div>
                    <div className="font-mono">{selectedEvent.ipAddress}</div>
                  </div>

                  <div className="win-border-groove p-2">
                    <div className="text-win-small text-muted-foreground mb-1">User Agent</div>
                    <div className="text-win-small break-all">
                      {selectedEvent.userAgent.slice(0, 100)}...
                    </div>
                  </div>

                  {Object.keys(selectedEvent.metadata).length > 0 && (
                    <div className="win-border-groove p-2">
                      <div className="text-win-small text-muted-foreground mb-1">Metadata</div>
                      <pre className="font-mono text-win-small overflow-auto max-h-[80px] bg-input p-1">
                        {JSON.stringify(selectedEvent.metadata, null, 2)}
                      </pre>
                    </div>
                  )}
                </div>
              ) : (
                <div className="text-muted-foreground text-win-body py-4 text-center">
                  Select an event to view details.
                </div>
              )}
            </Panel>
          </div>
        </div>
      </div>
    </AppLayout>
  );
};

export default AuditPage;
