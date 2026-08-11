import { ReactNode, useState } from 'react';

interface DesktopIconProps {
  icon: ReactNode;
  label: string;
  onDoubleClick?: () => void;
}

export const DesktopIcon = ({ icon, label, onDoubleClick }: DesktopIconProps) => {
  const [isSelected, setIsSelected] = useState(false);

  return (
    <div
      className="flex flex-col items-center gap-1 p-1 cursor-pointer select-none w-[72px]"
      onClick={() => setIsSelected(!isSelected)}
      onDoubleClick={onDoubleClick}
    >
      <div className={`p-1 ${isSelected ? 'bg-primary/50' : ''}`}>{icon}</div>
      <span
        className={`win-icon-text ${isSelected ? 'bg-primary text-primary-foreground px-1' : ''}`}
      >
        {label}
      </span>
    </div>
  );
};
