import { useState, useEffect } from 'react';
import { Button } from './Button';
import { Menu } from './Menu';

interface TaskbarItem {
  id: string;
  title: string;
  isActive: boolean;
}

interface TaskbarProps {
  items: TaskbarItem[];
  onItemClick: (id: string) => void;
  onStartClick?: () => void;
}

export const Taskbar = ({ items, onItemClick, onStartClick }: TaskbarProps) => {
  const [time, setTime] = useState(new Date());
  const [showStartMenu, setShowStartMenu] = useState(false);

  useEffect(() => {
    const timer = setInterval(() => setTime(new Date()), 1000);
    return () => clearInterval(timer);
  }, []);

  const formatTime = (date: Date) => {
    return date.toLocaleTimeString('en-US', {
      hour: 'numeric',
      minute: '2-digit',
      hour12: true,
    });
  };

  return (
    <>
      {/* Start Menu */}
      {showStartMenu && (
        <div className="fixed bottom-[28px] left-0 z-50 animate-win-open">
          <Menu
            items={[
              { id: 'programs', label: 'Programs', icon: '📁' },
              { id: 'documents', label: 'Documents', icon: '📄' },
              { id: 'settings', label: 'Settings', icon: '⚙️' },
              { id: 'find', label: 'Find', icon: '🔍' },
              { id: 'help', label: 'Help', icon: '❓' },
              { id: 'run', label: 'Run...', icon: '▶️' },
              { type: 'separator' },
              { id: 'shutdown', label: 'Shut Down...', icon: '🔌' },
            ]}
            onSelect={(id) => {
              setShowStartMenu(false);
              if (id === 'programs' || id === 'settings') {
                onStartClick?.();
              }
            }}
          />
        </div>
      )}

      {/* Taskbar */}
      <div className="fixed bottom-0 left-0 right-0 win-taskbar flex items-center gap-1 z-40">
        <Button
          variant="start"
          onClick={() => setShowStartMenu(!showStartMenu)}
          className={showStartMenu ? 'win-border-sunken' : ''}
        >
          <WindowsLogo />
          <span>Start</span>
        </Button>

        <div className="h-[20px] w-[2px] mx-1 win-border-groove" />

        <div className="flex-1 flex gap-1 overflow-hidden">
          {items.map((item) => (
            <button
              key={item.id}
              className={`win-button min-w-[120px] max-w-[160px] text-left truncate px-2 text-win-body ${
                item.isActive ? 'win-border-sunken' : ''
              }`}
              onClick={() => onItemClick(item.id)}
            >
              {item.title}
            </button>
          ))}
        </div>

        <div className="win-border-sunken px-2 py-1 text-win-body min-w-[70px] text-center">
          {formatTime(time)}
        </div>
      </div>
    </>
  );
};

const WindowsLogo = () => (
  <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
    <rect x="1" y="1" width="6" height="6" fill="#FF0000" />
    <rect x="9" y="1" width="6" height="6" fill="#00FF00" />
    <rect x="1" y="9" width="6" height="6" fill="#0000FF" />
    <rect x="9" y="9" width="6" height="6" fill="#FFFF00" />
  </svg>
);
