import { ReactNode } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { useAuth } from '@/contexts/AuthContext';
import { Button } from '@/components/win95';
import { Shield, Database, Users, FileText, LogOut, Home, Settings } from 'lucide-react';

interface AppLayoutProps {
  children: ReactNode;
}

const navItems = [
  { path: '/', label: 'Dashboard', icon: Home },
  { path: '/vaults', label: 'Vaults', icon: Database },
  { path: '/audit', label: 'Audit Log', icon: FileText },
  { path: '/settings', label: 'Settings', icon: Settings },
];

export const AppLayout = ({ children }: AppLayoutProps) => {
  const { user, logout } = useAuth();
  const location = useLocation();

  const handleLogout = async () => {
    await logout();
  };

  return (
    <div className="min-h-screen bg-background flex flex-col">
      {/* Title Bar */}
      <header className="win-border-raised bg-background h-[24px] flex items-center px-2 gap-2">
        <Shield size={14} strokeWidth={1.5} />
        <span className="text-win-body font-semibold">Vault Console</span>
        <span className="text-win-body text-muted-foreground">— Secrets Management System</span>
        <div className="flex-1" />
        {user && <span className="text-win-body text-muted-foreground">{user.email}</span>}
      </header>

      {/* Menu Bar */}
      <nav className="win-border-raised bg-background border-t-0 flex items-center px-1 py-[2px] gap-0">
        {navItems.map((item) => {
          const isActive =
            location.pathname === item.path ||
            (item.path !== '/' && location.pathname.startsWith(item.path));

          return (
            <Link
              key={item.path}
              to={item.path}
              className={`flex items-center gap-1 px-3 py-1 text-win-body hover:bg-primary/10 ${
                isActive ? 'win-border-sunken bg-secondary' : ''
              }`}
            >
              <item.icon size={12} strokeWidth={1.5} />
              <span>{item.label}</span>
            </Link>
          );
        })}
        <div className="flex-1" />
        <Button className="!min-w-0 !px-2 !py-1 flex items-center gap-1" onClick={handleLogout}>
          <LogOut size={12} strokeWidth={1.5} />
          <span>Logout</span>
        </Button>
      </nav>

      {/* Main Content */}
      <main className="flex-1 p-win-sm overflow-auto">{children}</main>

      {/* Status Bar */}
      <footer className="win-border-raised bg-background h-[20px] flex items-center px-2 gap-4 border-t-0">
        <div className="win-border-sunken flex-1 px-2 py-[1px] text-win-small">Ready</div>
        <div className="win-border-sunken px-2 py-[1px] text-win-small w-[140px]">
          {new Date().toLocaleDateString()}
        </div>
        <div className="win-border-sunken px-2 py-[1px] text-win-small w-[80px] text-center">
          v1.0.0
        </div>
      </footer>
    </div>
  );
};
