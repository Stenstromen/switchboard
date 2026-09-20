import { useEffect, useMemo, useRef, useState } from "react";
import type { TunnelView } from "../types";
import { statusText } from "../types";
import { ContextMenu, type ContextMenuItem, type MenuState } from "./ContextMenu";
import { ChevronDownIcon, GearIcon, PlusIcon } from "./Icons";

type Props = {
  tunnels: TunnelView[];
  allTunnels: TunnelView[];
  selectedId: string | null;
  tagFilter: string | null;
  blurred?: boolean;
  onSelect: (id: string) => void;
  onAdd: () => void;
  onOptions: () => void;
  onTagFilter: (tag: string | null) => void;
  onContextAction: (action: string, tunnelId: string) => void;
};

export function Sidebar({
  tunnels,
  allTunnels,
  selectedId,
  tagFilter,
  blurred = false,
  onSelect,
  onAdd,
  onOptions,
  onTagFilter,
  onContextAction,
}: Props) {
  const [tagMenuOpen, setTagMenuOpen] = useState(false);
  const [menu, setMenu] = useState<MenuState>(null);
  const titleWrapRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!tagMenuOpen) return;
    const onDown = (e: MouseEvent) => {
      if (!titleWrapRef.current?.contains(e.target as Node)) setTagMenuOpen(false);
    };
    window.addEventListener("mousedown", onDown);
    return () => window.removeEventListener("mousedown", onDown);
  }, [tagMenuOpen]);

  const allTags = useMemo(() => {
    const set = new Set<string>();
    for (const t of allTunnels) {
      for (const tag of t.profile.tags ?? []) set.add(tag);
    }
    return [...set].sort((a, b) => a.localeCompare(b));
  }, [allTunnels]);

  const count = tunnels.length;
  const title = tagFilter ? `Tag: ${tagFilter}` : "All Tunnels";

  const openItemMenu = (e: React.MouseEvent, id: string) => {
    e.preventDefault();
    e.stopPropagation();
    onSelect(id);
    const view = allTunnels.find((t) => t.profile.id === id);
    const st = view?.status.Status;
    const connected = st === "connected" || st === "connecting";
    const pinned = !!view?.profile.pinned;
    const items: ContextMenuItem[] = [
      { kind: "item", id: "connect", label: "Connect", disabled: !view || connected },
      { kind: "item", id: "disconnect", label: "Disconnect", disabled: !view || !connected },
      { kind: "separator" },
      { kind: "item", id: "pin", label: pinned ? "Unpin" : "Pin" },
      { kind: "separator" },
      { kind: "item", id: "edit", label: "Edit Tunnel" },
      { kind: "separator" },
      { kind: "item", id: "new", label: "New Tunnel" },
      { kind: "item", id: "duplicate", label: "Duplicate Tunnel" },
      { kind: "separator" },
      { kind: "item", id: "delete", label: "Delete", danger: true },
    ];
    setMenu({ x: e.clientX, y: e.clientY, items, payload: id });
  };

  return (
    <aside
      className={`sidebar${blurred ? " is-blurred" : ""}`}
      onContextMenu={(e) => {
        if (blurred) {
          e.preventDefault();
          return;
        }
        if ((e.target as HTMLElement).closest(".tunnel-item")) return;
        e.preventDefault();
        setMenu({
          x: e.clientX,
          y: e.clientY,
          items: [{ kind: "item", id: "new", label: "New Tunnel" }],
        });
      }}
    >
      <div className="sidebar-header">
        <div className="sidebar-title-wrap" ref={titleWrapRef}>
          <button
            className="sidebar-title-btn"
            onClick={() => setTagMenuOpen((v) => !v)}
            title="Filter by tag"
          >
            <span className="sidebar-title">{title}</span>
            <ChevronDownIcon size={15} open={tagMenuOpen} />
          </button>
          <div className="sidebar-count">
            {count} {count === 1 ? "tunnel" : "tunnels"}
            {tagFilter ? " filtered" : ""}
          </div>

          {tagMenuOpen && (
            <div className="sidebar-tag-menu scrollable">
              <button
                className={`sidebar-tag-item${!tagFilter ? " is-active" : ""}`}
                onClick={() => {
                  onTagFilter(null);
                  setTagMenuOpen(false);
                }}
              >
                All Tunnels
              </button>
              {allTags.length === 0 ? (
                <div className="sidebar-tag-empty">No tags yet — add some on a tunnel</div>
              ) : (
                allTags.map((tag) => (
                  <button
                    key={tag}
                    className={`sidebar-tag-item${tagFilter === tag ? " is-active" : ""}`}
                    onClick={() => {
                      onTagFilter(tag);
                      setTagMenuOpen(false);
                    }}
                  >
                    {tag}
                  </button>
                ))
              )}
            </div>
          )}
        </div>
        <div className="sidebar-header-actions">
          <button className="icon-btn" onClick={onOptions} title="Settings" aria-label="Settings">
            <GearIcon />
          </button>
          <button className="icon-btn" onClick={onAdd} title="New Tunnel" aria-label="New Tunnel">
            <PlusIcon />
          </button>
        </div>
      </div>
      <div className="tunnel-list scrollable" aria-hidden={blurred}>
        {tunnels.map((t) => {
          const selected = t.profile.id === selectedId;
          const st = t.status.Status;
          return (
            <button
              key={t.profile.id}
              className={`tunnel-item${selected ? " is-selected" : ""}`}
              onClick={() => {
                if (blurred) return;
                onSelect(t.profile.id);
              }}
              onContextMenu={(e) => {
                if (blurred) {
                  e.preventDefault();
                  return;
                }
                openItemMenu(e, t.profile.id);
              }}
              tabIndex={blurred ? -1 : 0}
            >
              <span className={`status-dot ${st}`} />
              <span className="tunnel-item-body">
                <div className="tunnel-item-name">
                  {t.profile.pinned ? <PinBadge /> : null}
                  <span className="tunnel-item-name-text">{t.profile.name || "Unnamed"}</span>
                </div>
                <div className="tunnel-item-status">
                  <span>{statusText(t.status)}</span>
                  {(t.profile.tags?.length ?? 0) > 0 ? (
                    <span className="tunnel-item-tags"> · {t.profile.tags!.slice(0, 2).join(", ")}</span>
                  ) : null}
                </div>
              </span>
            </button>
          );
        })}
      </div>

      {menu && (
        <ContextMenu
          x={menu.x}
          y={menu.y}
          items={menu.items}
          onClose={() => setMenu(null)}
          onSelect={(id) => {
            if (id === "new") {
              onAdd();
              return;
            }
            if (menu.payload) onContextAction(id, menu.payload);
          }}
        />
      )}
    </aside>
  );
}

function PinBadge() {
  return (
    <svg className="pin-badge" width="11" height="11" viewBox="0 0 24 24" fill="currentColor" aria-hidden>
      <path d="M16.2 2.8c.4-.4 1-.4 1.4 0l3.6 3.6c.4.4.4 1 0 1.4l-1.1 1.1-5 5-1.2 1.2 2.2 2.2-1.4 1.4-2.2-2.2-1.5 1.5L8 21.5 2.5 16l3.5-3.5 1.5 1.5 1.2-1.2 5-5 1.1-1.1ZM7.5 14.1 5.4 16.2l2.4 2.4 2.1-2.1-2.4-2.4Z" />
    </svg>
  );
}
