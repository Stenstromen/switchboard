import { useMemo, useState } from "react";
import type { Forward, Profile, TunnelType, TunnelView } from "../types";
import { destination, statusLabel } from "../types";
import { TagEditor } from "./TagEditor";
import { ContextMenu, type ContextMenuItem, type MenuState } from "./ContextMenu";
import { ArrowRightIcon, DuplicateIcon, EditSlidersIcon, ForwardTypeIcon, HostIcon, SearchIcon } from "./Icons";

type Props = {
  view: TunnelView;
  allTags: string[];
  search: string;
  tagOpenSignal?: number;
  onSearch: (q: string) => void;
  onConnect: () => void;
  onDisconnect: () => void;
  onEdit: () => void;
  onDuplicate: () => void;
  onDelete: () => void;
  onDetails: () => void;
  onToggle: (patch: Partial<Profile>) => void;
  onTagsChange: (tags: string[]) => void;
};

export function TunnelDetail({
  view,
  allTags,
  search,
  tagOpenSignal = 0,
  onSearch,
  onConnect,
  onDisconnect,
  onEdit,
  onDuplicate,
  onDelete,
  onDetails,
  onToggle,
  onTagsChange,
}: Props) {
  const { profile: p, status } = view;
  const connected = status.Status === "connected" || status.Status === "connecting";
  const [menu, setMenu] = useState<MenuState>(null);

  const q = search.trim().toLowerCase();
  const match = useMemo(() => {
    if (!q) return () => true;
    return (text: string) => text.toLowerCase().includes(q);
  }, [q]);

  const showDynamic = !q || match("dynamic") || p.forwards.some((f) => f.type === "dynamic" && matchForward(f, q));
  const showLocal = !q || match("local") || p.forwards.some((f) => f.type === "local" && matchForward(f, q));
  const showRemote = !q || match("remote") || p.forwards.some((f) => f.type === "remote" && matchForward(f, q));
  const showRev = !q || match("reverse") || p.forwards.some((f) => f.type === "reverse-dynamic" && matchForward(f, q));

  const openMenu = (e: React.MouseEvent) => {
    e.preventDefault();
    const items: ContextMenuItem[] = [
      { kind: "item", id: "connect", label: "Connect", disabled: connected },
      { kind: "item", id: "disconnect", label: "Disconnect", disabled: !connected },
      { kind: "separator" },
      { kind: "item", id: "pin", label: p.pinned ? "Unpin" : "Pin" },
      { kind: "separator" },
      { kind: "item", id: "edit", label: "Edit Tunnel" },
      { kind: "separator" },
      { kind: "item", id: "duplicate", label: "Duplicate Tunnel" },
      { kind: "separator" },
      { kind: "item", id: "delete", label: "Delete", danger: true },
    ];
    setMenu({ x: e.clientX, y: e.clientY, items });
  };

  return (
    <div className="detail" onContextMenu={openMenu}>
      <div className="main-scroll scrollable">
        <div className="detail-header">
          <div>
            <h1 className="detail-title">{p.name || "Unnamed"}</h1>
            <TagEditor tags={p.tags ?? []} allTags={allTags} onChange={onTagsChange} openSignal={tagOpenSignal} />
          </div>
          <div className="search-field">
            <SearchIcon />
            <input
              placeholder="Search"
              value={search}
              onChange={(e) => onSearch(e.target.value)}
            />
          </div>
        </div>

        <div className="card conn-card">
          <div className="conn-dest">
            <HostIcon />
            <span>{destination(p) || "No host configured"}</span>
          </div>
          <button className="btn" onClick={connected ? onDisconnect : onConnect}>
            {connected ? "Disconnect" : "Connect"}
          </button>
          <div className="conn-status">
            <span className={`status-dot ${status.Status}`} />
            <span>{statusLabel(status.Status)}</span>
          </div>
          <button className="btn btn-ghost" title="Connection details" onClick={onDetails}>
            Details…
          </button>
          {status.Err ? <div className="err-text" style={{ gridColumn: "1 / -1" }}>{status.Err}</div> : null}
        </div>

        <div className="options-list">
          <div className="option-row">
            <label>Send notification when connection status has changed</label>
            <button
              className={`toggle${p.notifyOnStatusChange ? " is-on" : ""}`}
              onClick={() => onToggle({ notifyOnStatusChange: !p.notifyOnStatusChange })}
              aria-label="Toggle notifications"
            />
          </div>
          <div className="option-row">
            <label>Automatically connect on startup</label>
            <button
              className={`toggle${p.autoConnect ? " is-on" : ""}`}
              onClick={() => onToggle({ autoConnect: !p.autoConnect })}
              aria-label="Toggle auto-connect"
            />
          </div>
        </div>

        {showDynamic && (
          <ForwardSection
            title="Dynamic Port Forwarding"
            type="dynamic"
            forwards={p.forwards.filter((f) => f.type === "dynamic" && (!q || matchForward(f, q) || match("dynamic")))}
            empty="No configured Dynamic Port Forwarding."
            dynamic
          />
        )}
        {showLocal && (
          <ForwardSection
            title="Local Port Forwarding"
            type="local"
            forwards={p.forwards.filter((f) => f.type === "local" && (!q || matchForward(f, q) || match("local")))}
            empty="No configured Local Port Forwarding."
          />
        )}
        {showRemote && (
          <ForwardSection
            title="Remote Port Forwarding"
            type="remote"
            forwards={p.forwards.filter((f) => f.type === "remote" && (!q || matchForward(f, q) || match("remote")))}
            empty="No configured Remote Port Forwarding."
          />
        )}
        {showRev && (
          <ForwardSection
            title="Reverse Dynamic Port Forwarding"
            type="reverse-dynamic"
            forwards={p.forwards.filter((f) => f.type === "reverse-dynamic" && (!q || matchForward(f, q) || match("reverse")))}
            empty="No configured Reverse Dynamic Port Forwarding."
            dynamic
          />
        )}
      </div>

      <div className="footer-bar">
        <div className="left">
          <button className="btn btn-icon-label" onClick={onDuplicate}>
            <DuplicateIcon />
            Duplicate
          </button>
        </div>
        <div className="right">
          <button className="btn btn-icon-label" onClick={onEdit}>
            <EditSlidersIcon />
            Edit
          </button>
        </div>
      </div>

      {menu && (
        <ContextMenu
          x={menu.x}
          y={menu.y}
          items={menu.items}
          onClose={() => setMenu(null)}
          onSelect={(id) => {
            if (id === "connect") connected ? onDisconnect() : onConnect();
            if (id === "disconnect") onDisconnect();
            if (id === "pin") onToggle({ pinned: !p.pinned });
            if (id === "edit") onEdit();
            if (id === "duplicate") onDuplicate();
            if (id === "details") onDetails();
            if (id === "delete") onDelete();
          }}
        />
      )}
    </div>
  );
}

function matchForward(f: Forward, q: string): boolean {
  const hay = `${f.localHost} ${f.localPort} ${f.remoteHost} ${f.remotePort} ${f.type}`.toLowerCase();
  return hay.includes(q);
}

function ForwardSection({
  title,
  type,
  forwards,
  empty,
  dynamic,
}: {
  title: string;
  type: TunnelType;
  forwards: Forward[];
  empty: string;
  dynamic?: boolean;
}) {
  return (
    <section className="section">
      <div className="section-header">
        <ForwardTypeIcon type={type} size={18} />
        <span>{title}</span>
      </div>
      {forwards.length === 0 ? (
        <div className="forward-empty">{empty}</div>
      ) : (
        <div className="forward-table">
          {!dynamic && (
            <div className="forward-head">
              <span>From This Mac</span>
              <span />
              <span>To Target Host</span>
            </div>
          )}
          {forwards.map((f) => (
            <div className="forward-row" key={f.id}>
              <span className="endpoint">
                {(f.localHost || "127.0.0.1") + " : " + f.localPort}
              </span>
              <span className="forward-arrow">
                <ArrowRightIcon size={13} />
              </span>
              <span className="endpoint">
                {dynamic ? "SOCKS" : `${f.remoteHost} : ${f.remotePort}`}
              </span>
            </div>
          ))}
        </div>
      )}
    </section>
  );
}
