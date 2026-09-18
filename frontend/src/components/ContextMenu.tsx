import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";

export type ContextMenuItem =
  | { kind: "item"; id: string; label: string; danger?: boolean; disabled?: boolean; shortcut?: string }
  | { kind: "separator" };

type Props = {
  x: number;
  y: number;
  items: ContextMenuItem[];
  onClose: () => void;
  onSelect: (id: string) => void;
};

export function ContextMenu({ x, y, items, onClose, onSelect }: Props) {
  const ref = useRef<HTMLDivElement>(null);
  const [pos, setPos] = useState({ left: x, top: y });

  useLayoutEffect(() => {
    const el = ref.current;
    if (!el) return;
    const rect = el.getBoundingClientRect();
    const pad = 8;
    setPos({
      left: Math.min(x, window.innerWidth - rect.width - pad),
      top: Math.min(y, window.innerHeight - rect.height - pad),
    });
  }, [x, y, items]);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    const onDown = (e: MouseEvent) => {
      if (!ref.current?.contains(e.target as Node)) onClose();
    };
    const onScroll = () => onClose();
    window.addEventListener("keydown", onKey);
    window.addEventListener("mousedown", onDown, true);
    window.addEventListener("scroll", onScroll, true);
    window.addEventListener("blur", onClose);
    return () => {
      window.removeEventListener("keydown", onKey);
      window.removeEventListener("mousedown", onDown, true);
      window.removeEventListener("scroll", onScroll, true);
      window.removeEventListener("blur", onClose);
    };
  }, [onClose]);

  return createPortal(
    <div
      ref={ref}
      className="context-menu"
      style={{ left: pos.left, top: pos.top }}
      role="menu"
    >
      {items.map((item, i) => {
        if (item.kind === "separator") {
          return <div key={`sep-${i}`} className="context-sep" />;
        }
        return (
          <button
            key={item.id}
            className={`context-item${item.danger ? " is-danger" : ""}`}
            disabled={item.disabled}
            role="menuitem"
            onMouseDown={(e) => {
              // Keep the menu alive until click; capture-phase outside
              // handlers must not see this as an outside press.
              e.preventDefault();
              e.stopPropagation();
            }}
            onClick={(e) => {
              e.preventDefault();
              e.stopPropagation();
              if (item.disabled) return;
              const id = item.id;
              onClose();
              // Defer so the menu unmounts before the action (e.g. confirm dialog).
              queueMicrotask(() => onSelect(id));
            }}
          >
            <span>{item.label}</span>
            {item.shortcut ? <span className="context-shortcut">{item.shortcut}</span> : null}
          </button>
        );
      })}
    </div>,
    document.body
  );
}

export type MenuState = { x: number; y: number; items: ContextMenuItem[]; payload?: string } | null;
