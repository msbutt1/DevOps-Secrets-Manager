import { useState } from 'react';
import { Link } from 'react-router-dom';
import { useAuth } from '@/contexts/AuthContext';
import { useCurrentOrganization } from '@/contexts/OrganizationContext';
import { canCreateVaults } from '@/lib/organizations';
import { Panel, Button } from '@/components/win95';
import { VaultFormDialog } from '@/components/VaultFormDialog';
import { AppLayout } from '@/components/AppLayout';
import { RoleBadge } from '@/components/RoleBadge';
import { useVaults, useCreateVault } from '@/hooks/use-vaults';
import { useAuditLogs } from '@/hooks/use-audit';
import { useDashboardAlerts, useDashboardStats, useHealth } from '@/hooks/use-dashboard';
import { formatUptime } from '@/lib/format';
import { LoadingState } from '@/components/LoadingState';
import { ErrorMessage } from '@/components/ErrorMessage';
import {
  Database,
  AlertTriangle,
  Clock,
  Activity,
  ChevronRight,
  Shield,
  Server,
  Key,
  RefreshCw,
  Users,
  Check,
} from 'lucide-react';
import { AUDIT_ACTIONS } from '@/types/api';
import type { DashboardAlertType, VaultCreateRequest, VaultRole } from '@/types/api';
import { isProductionEnvironment } from '@/lib/environments';

const actionLabels = new Map<string, string>(AUDIT_ACTIONS.map((a) => [a.value, a.label]));

const alertIcons: Record<DashboardAlertType, typeof AlertTriangle> = {
  secret_expired: AlertTriangle,
  secret_expiring: Clock,
  rotation_overdue: RefreshCw,
  member_inactive: Users,
};

const activityIcons: Record<string, typeof AlertTriangle> = {
  'secret.revealed': Key,
  'secret.updated': RefreshCw,
  'secret.created': Key,
  'secret.deleted': Key,
  'member.added': Users,
  'member.removed': Users,
  'member.role_changed': Users,
  'vault.created': Database,
  'vault.updated': Database,
  'vault.deleted': Database,
  'env.created': Server,
  'env.updated': Server,
  'env.deleted': Server,
};

// Helper function to format relative time
function formatRelativeTime(timestamp: string): string {
  const diff = Date.now() - new Date(timestamp).getTime();
  const minutes = Math.floor(diff / 60000);
  if (minutes < 60) return `${minutes} min ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  return `${Math.floor(hours / 24)}d ago`;
}

export const DashboardPage = () => {
  const { user } = useAuth();
  const [showCreateVault, setShowCreateVault] = useState(false);

  // Fetch real data
  const { currentOrganization } = useCurrentOrganization();
  const orgId = currentOrganization?.id;
  const { data: vaults = [], isLoading: vaultsLoading, error: vaultsError } = useVaults(orgId);
  // Recent changes and reveals; logins would crowd everything else out
  const { data: auditData, isLoading: auditLoading } = useAuditLogs({
    limit: 5,
    page: 1,
    excludeAction: ['login.success', 'login.failure'],
    organizationId: orgId,
  });
  const { data: stats, isLoading: statsLoading, error: statsError } = useDashboardStats(orgId);
  const { data: health, isError: healthError } = useHealth();
  const { data: alerts = [], isError: alertsError } = useDashboardAlerts(orgId);
  const createVaultMutation = useCreateVault();
  const apiHealthy = !healthError && health?.status === 'ok';

  // Transform audit events to activity format
  const recentActivity = (auditData?.data || []).map((event) => ({
    id: event.id,
    type: event.action,
    label: actionLabels.get(event.action) ?? event.action,
    target: event.targetName,
    user: event.userEmail,
    vault: event.vaultName,
    env: event.environmentName,
    timestamp: formatRelativeTime(event.timestamp),
  }));

  // Use first 3 vaults for display
  const displayVaults = vaults.slice(0, 3);

  // Loading state
  if (vaultsLoading || auditLoading || statsLoading) {
    return (
      <AppLayout>
        <LoadingState type="page" message="Loading dashboard..." />
      </AppLayout>
    );
  }

  // Error state
  if (vaultsError || statsError || !stats) {
    return (
      <AppLayout>
        <ErrorMessage error={vaultsError || statsError || new Error('Failed to load dashboard')} />
      </AppLayout>
    );
  }

  return (
    <AppLayout>
      <div className="space-y-win-sm">
        {/* Page Header */}
        <div className="win-border-raised bg-background p-2">
          <h1 className="text-win-title">System Dashboard</h1>
          <p className="text-win-body text-muted-foreground">
            Welcome back, {user?.name || user?.email}
          </p>
        </div>

        {/* Stats Row */}
        <div className="grid grid-cols-6 gap-win-sm">
          <Panel className="!p-2 text-center">
            <Key size={16} className="mx-auto mb-1 text-info" strokeWidth={1.5} />
            <div className="text-win-title font-semibold">{stats.secrets}</div>
            <div className="text-win-small text-muted-foreground">Total Secrets</div>
          </Panel>
          <Panel className="!p-2 text-center">
            <Database size={16} className="mx-auto mb-1 text-info" strokeWidth={1.5} />
            <div className="text-win-title font-semibold">{stats.vaults}</div>
            <div className="text-win-small text-muted-foreground">Vaults</div>
          </Panel>
          <Panel className="!p-2 text-center">
            <Server size={16} className="mx-auto mb-1 text-info" strokeWidth={1.5} />
            <div className="text-win-title font-semibold">{stats.environments}</div>
            <div className="text-win-small text-muted-foreground">Environments</div>
          </Panel>
          <Panel className="!p-2 text-center">
            <Clock size={16} className="mx-auto mb-1 text-warning" strokeWidth={1.5} />
            <div
              className={`text-win-title font-semibold ${stats.secretsExpired + stats.secretsExpiringSoon > 0 ? 'text-warning' : ''}`}
            >
              {stats.secretsExpired + stats.secretsExpiringSoon}
            </div>
            <div
              className="text-win-small text-muted-foreground"
              title={`${stats.secretsExpired} expired, ${stats.secretsExpiringSoon} expiring within ${stats.expiringSoonDays} days`}
            >
              Expired / Expiring
            </div>
          </Panel>
          <Panel className="!p-2 text-center">
            <RefreshCw size={16} className="mx-auto mb-1 text-warning" strokeWidth={1.5} />
            <div
              className={`text-win-title font-semibold ${stats.secretsRotationOverdue > 0 ? 'text-warning' : ''}`}
            >
              {stats.secretsRotationOverdue}
            </div>
            <div className="text-win-small text-muted-foreground">Rotation Overdue</div>
          </Panel>
          <Panel className="!p-2 text-center">
            <Users size={16} className="mx-auto mb-1 text-success" strokeWidth={1.5} />
            <div className="text-win-title font-semibold">
              {stats.activeUsers}
              <span className="text-win-small text-muted-foreground font-normal">
                {' '}
                / {stats.users}
              </span>
            </div>
            <div
              className="text-win-small text-muted-foreground"
              title={`People with access who logged in within ${stats.activeWindowDays} days`}
            >
              Active Users ({stats.activeWindowDays}d)
            </div>
          </Panel>
        </div>

        <div className="flex gap-win-sm">
          {/* Left Column - Vaults */}
          <div className="flex-1 space-y-win-sm">
            {/* Vaults Panel */}
            <Panel>
              <div className="flex items-center justify-between mb-2">
                <div className="flex items-center gap-2">
                  <Database size={14} strokeWidth={1.5} />
                  <h2 className="text-win-section font-semibold">Accessible Vaults</h2>
                </div>
                {canCreateVaults(currentOrganization?.role) && (
                  <Button
                    className="!min-w-0 !px-2 !py-1 text-win-small"
                    onClick={() => setShowCreateVault(true)}
                  >
                    + New Vault
                  </Button>
                )}
              </div>

              <div className="win-border-sunken bg-input">
                <table className="w-full text-win-body">
                  <thead>
                    <tr className="bg-secondary border-b border-border">
                      <th className="text-left px-2 py-1 font-semibold">Vault Name</th>
                      <th className="text-left px-2 py-1 font-semibold">Organization</th>
                      <th className="text-left px-2 py-1 font-semibold w-[100px]">Role</th>
                      <th className="text-left px-2 py-1 font-semibold w-[70px]">Secrets</th>
                      <th className="w-[40px]"></th>
                    </tr>
                  </thead>
                  <tbody>
                    {displayVaults.map((vault, idx) => (
                      <tr
                        key={vault.id}
                        className={`border-b border-border/50 hover:bg-primary/10 ${
                          idx % 2 === 1 ? 'bg-background' : ''
                        }`}
                      >
                        <td className="px-2 py-1">
                          <Link
                            to={`/vaults/${vault.id}`}
                            className="hover:underline text-info font-semibold"
                          >
                            {vault.name}
                          </Link>
                        </td>
                        <td className="px-2 py-1">{vault.organizationName}</td>
                        <td className="px-2 py-1">
                          <RoleBadge role={vault.userRole} size="sm" />
                        </td>
                        <td className="px-2 py-1 text-center">{vault.secretCount || 0}</td>
                        <td className="px-2 py-1">
                          <Link to={`/vaults/${vault.id}`}>
                            <ChevronRight size={14} strokeWidth={1.5} />
                          </Link>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>

              <div className="mt-2">
                <Link to="/vaults" className="text-win-small text-info hover:underline">
                  View all vaults
                </Link>
              </div>
            </Panel>
          </div>

          {/* Right Column - Status Panels */}
          <div className="w-[340px] space-y-win-sm">
            {/* Alerts Panel */}
            <Panel>
              <div className="flex items-center gap-2 mb-2">
                <AlertTriangle size={14} strokeWidth={1.5} className="text-warning" />
                <h2 className="text-win-section font-semibold">System Alerts</h2>
                <span className="text-win-small text-warning ml-auto">{alerts.length} active</span>
              </div>

              <div className="win-border-sunken bg-input max-h-[140px] overflow-auto">
                {alerts.map((alert) => {
                  const Icon = alertIcons[alert.type] ?? AlertTriangle;
                  const target =
                    alert.type === 'member_inactive'
                      ? `/vaults/${alert.vaultId}/access`
                      : `/vaults/${alert.vaultId}`;
                  return (
                    <Link
                      key={`${alert.type}-${alert.targetId}-${alert.vaultId}`}
                      to={target}
                      className="px-2 py-1 border-b border-border/50 text-win-body flex items-start gap-2 hover:bg-primary/10"
                    >
                      <Icon
                        size={12}
                        className={`mt-[2px] flex-shrink-0 ${alert.severity === 'low' ? 'text-muted-foreground' : 'text-warning'}`}
                        strokeWidth={1.5}
                      />
                      <div className="flex-1 min-w-0">
                        <div className="truncate">{alert.message}</div>
                        <div className="text-win-small text-muted-foreground truncate">
                          {alert.vaultName}
                          {alert.environmentName ? ` / ${alert.environmentName}` : ''}
                          {alert.dueAt && alert.type !== 'member_inactive'
                            ? ` — ${new Date(alert.dueAt).toLocaleDateString()}`
                            : ''}
                        </div>
                      </div>
                      <span
                        className={`text-win-small px-1 ${
                          alert.severity === 'high'
                            ? 'bg-warning/20 text-warning'
                            : alert.severity === 'medium'
                              ? 'bg-warning/10 text-warning'
                              : 'text-muted-foreground'
                        }`}
                      >
                        {alert.severity}
                      </span>
                    </Link>
                  );
                })}
                {alertsError && (
                  <div className="px-2 py-4 text-center text-warning">Could not load alerts</div>
                )}
                {!alertsError && alerts.length === 0 && (
                  <div className="px-2 py-4 text-center text-muted-foreground">
                    <Check size={16} className="mx-auto mb-1 text-success" />
                    No active alerts
                  </div>
                )}
              </div>
            </Panel>

            {/* Recent Activity Panel */}
            <Panel>
              <div className="flex items-center gap-2 mb-2">
                <Activity size={14} strokeWidth={1.5} />
                <h2 className="text-win-section font-semibold">Recent Activity</h2>
              </div>

              <div className="win-border-sunken bg-input max-h-[180px] overflow-auto">
                {recentActivity.map((activity) => {
                  const Icon = activityIcons[activity.type] || Activity;
                  return (
                    <div
                      key={activity.id}
                      className="px-2 py-1 border-b border-border/50 text-win-body"
                    >
                      <div className="flex items-center gap-2">
                        <Icon size={10} strokeWidth={1.5} className="flex-shrink-0" />
                        <span className="font-semibold truncate min-w-0">
                          {activity.label}
                          {activity.target && (
                            <span className="font-normal font-mono"> {activity.target}</span>
                          )}
                        </span>
                        <span className="text-muted-foreground text-win-small ml-auto whitespace-nowrap">
                          {activity.timestamp}
                        </span>
                      </div>
                      <div className="text-win-small text-muted-foreground pl-4">
                        {activity.user}
                        {activity.vault && ` in ${activity.vault}`}
                        {activity.vault && activity.env && (
                          <span
                            className={isProductionEnvironment(activity.env) ? 'text-warning' : ''}
                          >
                            /{activity.env}
                          </span>
                        )}
                      </div>
                    </div>
                  );
                })}
                {recentActivity.length === 0 && (
                  <div className="px-2 py-4 text-center text-muted-foreground text-win-body">
                    No recent activity
                  </div>
                )}
              </div>

              <div className="mt-2">
                <Link to="/audit" className="text-win-small text-info hover:underline">
                  View full audit log
                </Link>
              </div>
            </Panel>

            {/* System Status Panel */}
            <Panel>
              <div className="flex items-center gap-2 mb-2">
                <Shield size={14} strokeWidth={1.5} className="text-success" />
                <h2 className="text-win-section font-semibold">System Status</h2>
              </div>

              <div className="space-y-1 text-win-body">
                <div className="flex justify-between items-center">
                  <span>API Status:</span>
                  {apiHealthy ? (
                    <span className="text-success font-semibold flex items-center gap-1">
                      <span className="w-2 h-2 rounded-full bg-success" />
                      Operational
                    </span>
                  ) : (
                    <span className="text-warning font-semibold flex items-center gap-1">
                      <span className="w-2 h-2 rounded-full bg-warning" />
                      {health ? 'Degraded' : 'Unreachable'}
                    </span>
                  )}
                </div>
                <div className="flex justify-between">
                  <span>Database:</span>
                  <span className="font-semibold">{health?.database ?? 'unknown'}</span>
                </div>
                <div className="flex justify-between">
                  <span>Encryption:</span>
                  <span className="font-semibold">AES-256-GCM envelope</span>
                </div>
                <div className="flex justify-between">
                  <span>Schema Version:</span>
                  <span className="text-muted-foreground">{health?.migrationVersion ?? '—'}</span>
                </div>
                <div className="flex justify-between">
                  <span>API Uptime:</span>
                  <span className="text-muted-foreground">
                    {health ? formatUptime(health.uptimeSeconds) : '—'}
                  </span>
                </div>
                <div className="flex justify-between">
                  <span>API Version:</span>
                  <span className="text-muted-foreground">{health?.version ?? '—'}</span>
                </div>
              </div>
            </Panel>
          </div>
        </div>
      </div>

      {/* Create Vault Dialog */}
      <VaultFormDialog
        isOpen={showCreateVault}
        onClose={() => setShowCreateVault(false)}
        onSave={async (data) => {
          await createVaultMutation.mutateAsync({
            ...(data as VaultCreateRequest),
            organizationId: orgId,
          });
          setShowCreateVault(false);
        }}
      />
    </AppLayout>
  );
};

export default DashboardPage;
