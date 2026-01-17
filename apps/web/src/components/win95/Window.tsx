import { ReactNode, useState, useRef, useEffect } from "react";
import { Minus, Square, X } from "lucide-react";

interface WindowProps {
  title: string;
  children: ReactNode;
  isActive?: boolean;
  onClose?: () => void;
  onMinimize?: () => void;
  onMaximize?: () => void;
  defaultPosition?: { x: number; y: number };
  defaultSize?: { width: number; height: number };
  showMinimize?: boolean;
  showMaximize?: boolean;
  showClose?: boolean;
  className?: string;
  zIndex?: number;
  onFocus?: () => void;
}

export const Window = ({
  title,
  children,
  isActive = true,
  onClose,
  onMinimize,
  onMaximize,
  defaultPosition = { x: 100, y: 50 },
  defaultSize = { width: 400, height: 300 },
  showMinimize = true,
  showMaximize = true,
  showClose = true,
  className = "",
  zIndex = 10,
  onFocus,
}: WindowProps) => {
  const [position, setPosition] = useState(defaultPosition);
  const [size, setSize] = useState(defaultSize);
  const [isDragging, setIsDragging] = useState(false);
  const [isMaximized, setIsMaximized] = useState(false);
  const [isClosing, setIsClosing] = useState(false);
  const [isMinimizing, setIsMinimizing] = useState(false);
  const dragOffset = useRef({ x: 0, y: 0 });
  const prevState = useRef({ position, size });

  useEffect(() => {
    const handleMouseMove = (e: MouseEvent) => {
      if (isDragging && !isMaximized) {
        setPosition({
          x: e.clientX - dragOffset.current.x,
          y: Math.max(0, e.clientY - dragOffset.current.y),
        });
      }
    };

    const handleMouseUp = () => {
      setIsDragging(false);
    };

    if (isDragging) {
      window.addEventListener("mousemove", handleMouseMove);
      window.addEventListener("mouseup", handleMouseUp);
    }

    return () => {
      window.removeEventListener("mousemove", handleMouseMove);
      window.removeEventListener("mouseup", handleMouseUp);
    };
  }, [isDragging, isMaximized]);

  const handleTitleMouseDown = (e: React.MouseEvent) => {
    if (isMaximized) return;
    dragOffset.current = {
      x: e.clientX - position.x,
      y: e.clientY - position.y,
    };
    setIsDragging(true);
    onFocus?.();
  };

  const handleMaximize = () => {
    if (isMaximized) {
      setPosition(prevState.current.position);
      setSize(prevState.current.size);
    } else {
      prevState.current = { position, size };
      setPosition({ x: 0, y: 0 });
      setSize({ width: window.innerWidth, height: window.innerHeight - 28 });
    }
    setIsMaximized(!isMaximized);
    onMaximize?.();
  };

  const handleClose = () => {
    setIsClosing(true);
    setTimeout(() => {
      onClose?.();
    }, 100);
  };

  const handleMinimize = () => {
    setIsMinimizing(true);
    setTimeout(() => {
      onMinimize?.();
      setIsMinimizing(false);
    }, 200);
  };

  const windowStyle = isMaximized
    ? { left: 0, top: 0, width: "100%", height: "calc(100vh - 28px)", zIndex }
    : { left: position.x, top: position.y, width: size.width, height: size.height, zIndex };

  return (
    <div
      className={`fixed win-border-raised bg-background flex flex-col ${className} 
        ${isClosing ? "animate-win-close" : "animate-win-open"}
        ${isMinimizing ? "animate-win-minimize" : ""}`}
      style={windowStyle}
      onMouseDown={onFocus}
    >
      {/* Title Bar */}
      <div
        className={`win-title-bar ${!isActive ? "win-title-bar-inactive" : ""} cursor-move select-none`}
        onMouseDown={handleTitleMouseDown}
        onDoubleClick={handleMaximize}
      >
        <span className="truncate">{title}</span>
        <div className="flex gap-[2px]">
          {showMinimize && (
            <button
              className="win-title-button"
              onClick={(e) => {
                e.stopPropagation();
                handleMinimize();
              }}
            >
              <Minus size={8} strokeWidth={3} />
            </button>
          )}
          {showMaximize && (
            <button
              className="win-title-button"
              onClick={(e) => {
                e.stopPropagation();
                handleMaximize();
              }}
            >
              <Square size={8} strokeWidth={2} />
            </button>
          )}
          {showClose && (
            <button
              className="win-title-button"
              onClick={(e) => {
                e.stopPropagation();
                handleClose();
              }}
            >
              <X size={10} strokeWidth={3} />
            </button>
          )}
        </div>
      </div>

      {/* Content */}
      <div className="flex-1 overflow-auto p-win-xs">{children}</div>
    </div>
  );
};
