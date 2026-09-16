import { ReactNode } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { useAuth } from '@/contexts/AuthContext';
import { Button } from '@/components/win95';
import { OrganizationSwitcher } from '@/components/OrganizationSwitcher';
import { GlobalSearch } from '@/components/GlobalSearch';
import {
  Shield,
  Database,
  Building,
  FileText,
  LogOut,
  Home,
  Settings,
  AlertTriangle,
} from 'lucide-react';

interface AppLayoutProps {
  children: ReactNode;
}

const navItems = [
  { path: '/', label: 'Dashboard', icon: Home },
  { path: '/vaults', label: 'Vaults', icon: Database },
  { path: '/audit', label: 'Audit Log', icon: FileText },
  { path: '/organization', label: 'Organization', icon: Building },
  { path: '/settings', label: 'Settings', icon: Settings },
];

const demoBanner = import.meta.env.VITE_DEMO_BANNER;

export const AppLayout = ({ children }: AppLayoutProps) => {
  const { user, logout } = useAuth();
  const location = useLocation();

  const handleLogout = async () => {
    await logout();
  };

  return (
    <div className="min-h-screen w-full max-w-full overflow-x-hidden bg-background flex flex-col">
      {/* Title Bar */}
      <header className="win-border-raised bg-background min-h-[24px] flex w-full items-center px-2 gap-2">
        <Shield size={14} strokeWidth={1.5} className="shrink-0" />
        <span className="text-win-body font-semibold whitespace-nowrap">Vault Console</span>
        <span className="hidden md:inline text-win-body text-muted-foreground">
          — Secrets Management System
        </span>
        <div className="flex-1" />
        <OrganizationSwitcher />
        {user && (
          <span className="hidden sm:inline text-win-body text-muted-foreground truncate max-w-[180px]">
            {user.email}
          </span>
        )}
      </header>

      {/* Menu Bar */}
      <nav className="win-border-raised bg-background border-t-0 flex w-full items-center px-1 py-[2px] gap-0 overflow-x-auto">
        {navItems.map((item) => {
          const isActive =
            location.pathname === item.path ||
            (item.path !== '/' && location.pathname.startsWith(item.path));

          return (
            <Link
              key={item.path}
              to={item.path}
              className={`flex shrink-0 items-center gap-1 px-3 py-1 text-win-body hover:bg-primary/10 ${
                isActive ? 'win-border-sunken bg-secondary' : ''
              }`}
            >
              <item.icon size={12} strokeWidth={1.5} />
              <span>{item.label}</span>
            </Link>
          );
        })}
        <div className="flex-1" />
        <div className="hidden sm:block">
          <GlobalSearch />
        </div>
        <Button
          className="!min-w-0 !px-2 !py-1 flex shrink-0 items-center gap-1 ml-2"
          onClick={handleLogout}
        >
          <LogOut size={12} strokeWidth={1.5} />
          <span>Logout</span>
        </Button>
      </nav>

      {/* A demo deployment says so: see docs/operations.md */}
      {demoBanner && (
        <div
          role="status"
          className="win-border-raised border-t-0 bg-warning/15 px-2 py-1 text-win-small flex items-center gap-2"
        >
          <AlertTriangle size={12} className="text-warning shrink-0" strokeWidth={1.5} />
          <span>{demoBanner}</span>
        </div>
      )}

      {/* Main Content */}
      <main className="flex-1 w-full min-w-0 p-win-sm overflow-auto">{children}</main>

      {/* Status Bar */}
      <footer className="win-border-raised bg-background min-h-[20px] flex w-full items-center px-2 gap-4 border-t-0">
        <div className="win-border-sunken flex-1 px-2 py-[1px] text-win-small">Ready</div>
        <div className="hidden sm:block win-border-sunken px-2 py-[1px] text-win-small w-[140px]">
          {new Date().toLocaleDateString()}
        </div>
        <div className="win-border-sunken px-2 py-[1px] text-win-small w-[80px] text-center">
          v1.0.0
        </div>
      </footer>
    </div>
  );
};
