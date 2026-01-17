interface MenuItem {
  id?: string;
  label?: string;
  icon?: string;
  type?: "separator";
  disabled?: boolean;
}

interface MenuProps {
  items: MenuItem[];
  onSelect: (id: string) => void;
}

export const Menu = ({ items, onSelect }: MenuProps) => {
  return (
    <div className="win-menu">
      {items.map((item, index) =>
        item.type === "separator" ? (
          <div key={index} className="win-menu-separator" />
        ) : (
          <div
            key={item.id || index}
            className={`win-menu-item ${item.disabled ? "text-disabled" : ""}`}
            onClick={() => !item.disabled && item.id && onSelect(item.id)}
          >
            <span className="w-5 text-center">{item.icon}</span>
            <span>{item.label}</span>
          </div>
        )
      )}
    </div>
  );
};
