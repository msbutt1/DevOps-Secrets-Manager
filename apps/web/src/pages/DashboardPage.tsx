import { useState } from 'react';
import { Link } from 'react-router-dom';
import { useAuth } from '@/contexts/AuthContext';
import { Panel, Button } from '@/components/win95';
import { VaultFormDialog } from '@/components/VaultFormDialog';
import { AppLayout } from '@/components/AppLayout';
import { RoleBadge } from '@/components/RoleBadge';
import { useVaults, useCreateVault } from '@/hooks/use-vaults';
import { useAuditLogs } from '@/hooks/use-audit';
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
import type { VaultCreateRequest, VaultRole } from '@/types/api';
import { isProductionEnvironment } from '@/lib/environments';

// Alerts will show real data once backend supports expiration/rotation tracking
const alerts: { id: string; type: string; message: string; vault: string; severity: string }[] = [];

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
  const { data: vaults = [], isLoading: vaultsLoading, error: vaultsError } = useVaults();
  const { data: auditData, isLoading: auditLoading } = useAuditLogs({
    limit: 4,
    page: 1,
  });
  const createVaultMutation = useCreateVault();

  // Transform audit events to activity format
  const recentActivity = (auditData?.data || []).map((event) => ({
    id: event.id,
    type: event.action,
    user: event.userEmail,
    vault: event.vaultName || 'System',
    env: event.environmentName,
    timestamp: formatRelativeTime(event.timestamp),
  }));

  // Use first 3 vaults for display
  const displayVaults = vaults.slice(0, 3);

  // Compute real statistics
  const stats = {
    totalVaults: vaults.length,
    totalSecrets: vaults.reduce((sum, v) => sum + (v.secretCount || 0), 0),
    totalEnvironments: vaults.reduce((sum, v) => sum + (v.envCount || 0), 0),
    activeUsers: 0, // Placeholder - backend doesn't track this
    secretsExpiringSoon: 0, // Placeholder - backend doesn't support
    secretsNeedingRotation: 0, // Placeholder - backend doesn't support
  };

  // Loading state
  if (vaultsLoading || auditLoading) {
    return (
      <AppLayout>
        <LoadingState type="page" message="Loading dashboard..." />
      </AppLayout>
    );
  }

  // Error state
  if (vaultsError) {
    return (
      <AppLayout>
        <ErrorMessage error={vaultsError} />
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
            <div className="text-win-title font-semibold">{stats.totalSecrets}</div>
            <div className="text-win-small text-muted-foreground">Total Secrets</div>
          </Panel>
          <Panel className="!p-2 text-center">
            <Database size={16} className="mx-auto mb-1 text-info" strokeWidth={1.5} />
            <div className="text-win-title font-semibold">{stats.totalVaults}</div>
            <div className="text-win-small text-muted-foreground">Vaults</div>
          </Panel>
          <Panel className="!p-2 text-center">
            <Server size={16} className="mx-auto mb-1 text-info" strokeWidth={1.5} />
            <div className="text-win-title font-semibold">{stats.totalEnvironments}</div>
            <div className="text-win-small text-muted-foreground">Environments</div>
          </Panel>
          <Panel className="!p-2 text-center">
            <Clock size={16} className="mx-auto mb-1 text-warning" strokeWidth={1.5} />
            <div className="text-win-title font-semibold text-warning">
              {stats.secretsExpiringSoon}
            </div>
            <div className="text-win-small text-muted-foreground">Expiring Soon</div>
          </Panel>
          <Panel className="!p-2 text-center">
            <RefreshCw size={16} className="mx-auto mb-1 text-warning" strokeWidth={1.5} />
            <div className="text-win-title font-semibold text-warning">
              {stats.secretsNeedingRotation}
            </div>
            <div className="text-win-small text-muted-foreground">Need Rotation</div>
          </Panel>
          <Panel className="!p-2 text-center">
            <Users size={16} className="mx-auto mb-1 text-success" strokeWidth={1.5} />
            <div className="text-win-title font-semibold">{stats.activeUsers}</div>
            <div className="text-win-small text-muted-foreground">Active Users</div>
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
                <Button
                  className="!min-w-0 !px-2 !py-1 text-win-small"
                  onClick={() => setShowCreateVault(true)}
                >
                  + New Vault
                </Button>
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
                {alerts.map((alert) => (
                  <div
                    key={alert.id}
                    className="px-2 py-1 border-b border-border/50 text-win-body flex items-start gap-2"
                  >
                    {alert.type === 'expiring' ? (
                      <Clock
                        size={12}
                        className="text-warning mt-[2px] flex-shrink-0"
                        strokeWidth={1.5}
                      />
                    ) : (
                      <RefreshCw
                        size={12}
                        className="text-warning mt-[2px] flex-shrink-0"
                        strokeWidth={1.5}
                      />
                    )}
                    <div className="flex-1 min-w-0">
                      <div className="truncate">{alert.message}</div>
                      <div className="text-win-small text-muted-foreground">{alert.vault}</div>
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
                  </div>
                ))}
                {alerts.length === 0 && (
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
                        <span className="font-semibold">{activity.type}</span>
                        <span className="text-muted-foreground text-win-small ml-auto">
                          {activity.timestamp}
                        </span>
                      </div>
                      <div className="text-win-small text-muted-foreground pl-4">
                        {activity.user} in {activity.vault}
                        {activity.env && (
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
                  <span className="text-success font-semibold flex items-center gap-1">
                    <span className="w-2 h-2 rounded-full bg-success animate-pulse" />
                    Operational
                  </span>
                </div>
                <div className="flex justify-between">
                  <span>Encryption:</span>
                  <span className="font-semibold">AES-256-GCM</span>
                </div>
                <div className="flex justify-between">
                  <span>Key Derivation:</span>
                  <span className="font-semibold">PBKDF2</span>
                </div>
                <div className="flex justify-between">
                  <span>Last Backup:</span>
                  <span className="text-muted-foreground">2 hours ago</span>
                </div>
                <div className="flex justify-between">
                  <span>Uptime:</span>
                  <span className="text-muted-foreground">99.99%</span>
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
            organizationId: user?.organizations?.[0]?.id,
          });
          setShowCreateVault(false);
        }}
      />
    </AppLayout>
  );
};

export default DashboardPage;
