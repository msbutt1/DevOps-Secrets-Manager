import type { VaultRole } from '@/types/api';

interface RoleBadgeProps {
  role: VaultRole;
  size?: 'sm' | 'md';
}

const roleStyles: Record<VaultRole, { bg: string; text: string; label: string }> = {
  owner: {
    bg: 'bg-info/10',
    text: 'text-info',
    label: 'Owner',
  },
  admin: {
    bg: 'bg-info/10',
    text: 'text-info',
    label: 'Admin',
  },
  developer: {
    bg: 'bg-success/10',
    text: 'text-success',
    label: 'Developer',
  },
  oncall: {
    bg: 'bg-warning/10',
    text: 'text-warning',
    label: 'On-Call',
  },
  viewer: {
    bg: 'bg-muted/30',
    text: 'text-muted-foreground',
    label: 'Viewer',
  },
};

export const RoleBadge = ({ role, size = 'md' }: RoleBadgeProps) => {
  const style = roleStyles[role] || roleStyles.viewer;

  return (
    <span
      className={`
        inline-block font-semibold 
        ${style.bg} ${style.text}
        ${size === 'sm' ? 'text-win-small px-1 py-[1px]' : 'text-win-body px-2 py-[2px]'}
      `}
    >
      {style.label}
    </span>
  );
};
