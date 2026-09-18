import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Events } from "@wailsio/runtime";
import {
  connectTunnel,
  deleteTunnel,
  disconnectTunnel,
  listTunnels,
  newTunnelDraft,
  openSettings,
  saveTunnel,
  setSelection,
  submitPassword,
} from "./api";
import { Sidebar } from "./components/Sidebar";
import { TunnelDetail } from "./components/TunnelDetail";
import { TunnelEditor } from "./components/TunnelEditor";
import { PasswordModal } from "./components/PasswordModal";
import { DetailsModal } from "./components/DetailsModal";
import { ConfirmDialog } from "./components/ConfirmDialog";
import { SettingsPage } from "./components/SettingsPage";
import type { PasswordPrompt, Profile, StatusEvent, TunnelView } from "./types";
import "./styles.css";

const isSettingsWindow =
  typeof window !== "undefined" &&
  new URLSearchParams(window.location.search).get("page") === "settings";

const SIDEBAR_MIN = 180;
const SIDEBAR_MAX = 420;
const SIDEBAR_DEFAULT = 240;
const SIDEBAR_STORAGE_KEY = "switchboard.sidebarWidth";

type Mode =
  | { kind: "browse" }
  | { kind: "create"; draft: Profile }
  | { kind: "edit"; draft: Profile };

type PendingDelete = { id: string; name: string };

export default function App() {
  if (isSettingsWindow) {
    return <SettingsPage />;
  }
  return <MainApp />;
}

function readSidebarWidth(): number {
  try {
    const raw = localStorage.getItem(SIDEBAR_STORAGE_KEY);
    const n = raw ? Number(raw) : SIDEBAR_DEFAULT;
    if (Number.isFinite(n)) return Math.min(SIDEBAR_MAX, Math.max(SIDEBAR_MIN, n));
  } catch {
    /* ignore */
  }
  return SIDEBAR_DEFAULT;
}

function MainApp() {
  const [tunnels, setTunnels] = useState<TunnelView[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [mode, setMode] = useState<Mode>({ kind: "browse" });
  const [passwordPrompt, setPasswordPrompt] = useState<PasswordPrompt | null>(null);
  const [detailsOpen, setDetailsOpen] = useState(false);
  const [pendingDelete, setPendingDelete] = useState<PendingDelete | null>(null);
  const [tagFilter, setTagFilter] = useState<string | null>(null);
  const [search, setSearch] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [sidebarWidth, setSidebarWidth] = useState(readSidebarWidth);
  const [sidebarVisible, setSidebarVisible] = useState(true);
  const [tagOpenSignal, setTagOpenSignal] = useState(0);
  const dragRef = useRef<{ startX: number; startW: number } | null>(null);

  const editing = mode.kind === "create" || mode.kind === "edit";

  const onSidebarResizeStart = useCallback(
    (e: React.MouseEvent) => {
      e.preventDefault();
      dragRef.current = { startX: e.clientX, startW: sidebarWidth };
      const onMove = (ev: MouseEvent) => {
        const drag = dragRef.current;
        if (!drag) return;
        const next = Math.min(SIDEBAR_MAX, Math.max(SIDEBAR_MIN, drag.startW + (ev.clientX - drag.startX)));
        setSidebarWidth(next);
      };
      const onUp = () => {
        dragRef.current = null;
        window.removeEventListener("mousemove", onMove);
        window.removeEventListener("mouseup", onUp);
        document.body.classList.remove("is-resizing-col");
        setSidebarWidth((w) => {
          try {
            localStorage.setItem(SIDEBAR_STORAGE_KEY, String(w));
          } catch {
            /* ignore */
          }
          return w;
        });
      };
      document.body.classList.add("is-resizing-col");
      window.addEventListener("mousemove", onMove);
      window.addEventListener("mouseup", onUp);
    },
    [sidebarWidth]
  );

  useEffect(() => {
    return () => document.body.classList.remove("is-resizing-col");
  }, []);

  const refresh = useCallback(async () => {
    try {
      const list = await listTunnels();
      setTunnels(list);
      setSelectedId((cur) => {
        if (cur && list.some((t) => t.profile.id === cur)) return cur;
        return list[0]?.profile.id ?? null;
      });
      setError(null);
    } catch (e: any) {
      setError(String(e?.message ?? e));
    }
  }, []);

  useEffect(() => {
    refresh();
    const offStatus = Events.On("tunnel:status", (ev: any) => {
      const data = (ev?.data ?? ev) as StatusEvent;
      if (!data?.hostId) return;
      setTunnels((prev) =>
        prev.map((t) =>
          t.profile.id === data.hostId ? { ...t, status: data.snap } : t
        )
      );
    });
    const offPassword = Events.On("tunnel:password", (ev: any) => {
      const data = (ev?.data ?? ev) as PasswordPrompt;
      if (data?.requestId) setPasswordPrompt(data);
    });
    const offConfig = Events.On("config:changed", () => {
      void refresh();
    });
    return () => {
      offStatus?.();
      offPassword?.();
      offConfig?.();
    };
  }, [refresh]);

  const allTags = useMemo(() => {
    const set = new Set<string>();
    for (const t of tunnels) {
      for (const tag of t.profile.tags ?? []) set.add(tag);
    }
    return [...set].sort((a, b) => a.localeCompare(b));
  }, [tunnels]);

  const visibleTunnels = useMemo(() => {
    const list = !tagFilter
      ? tunnels
      : tunnels.filter((t) => (t.profile.tags ?? []).includes(tagFilter));
    return [...list].sort((a, b) => {
      const ap = a.profile.pinned ? 1 : 0;
      const bp = b.profile.pinned ? 1 : 0;
      if (ap !== bp) return bp - ap;
      return 0;
    });
  }, [tunnels, tagFilter]);

  const selected = tunnels.find((t) => t.profile.id === selectedId) ?? null;

  useEffect(() => {
    if (selectedId && !visibleTunnels.some((t) => t.profile.id === selectedId)) {
      setSelectedId(visibleTunnels[0]?.profile.id ?? null);
    }
  }, [visibleTunnels, selectedId]);

  useEffect(() => {
    void setSelection(selectedId ?? "");
  }, [selectedId, selected?.status.Status, selected?.profile.pinned]);

  const startCreate = async () => {
    if (editing) return;
    const draft = await newTunnelDraft();
    setMode({ kind: "create", draft });
  };

  const startEdit = (id?: string) => {
    if (editing) return;
    const view = tunnels.find((t) => t.profile.id === (id ?? selectedId));
    if (!view) return;
    setSelectedId(view.profile.id);
    setMode({ kind: "edit", draft: { ...view.profile } });
  };

  const handleSave = async (p: Profile) => {
    try {
      const view = await saveTunnel(p);
      setMode({ kind: "browse" });
      await refresh();
      setSelectedId(view.profile.id);
    } catch (e: any) {
      setError(String(e?.message ?? e));
    }
  };

  const handleDuplicate = async (id?: string) => {
    const view = tunnels.find((t) => t.profile.id === (id ?? selectedId));
    if (!view) return;
    const draft = await newTunnelDraft();
    setMode({
      kind: "create",
      draft: {
        ...view.profile,
        id: draft.id,
        name: `${view.profile.name} Copy`,
      },
    });
  };

  const requestDelete = (id?: string) => {
    const target = id ?? selectedId;
    if (!target) return;
    const view = tunnels.find((t) => t.profile.id === target);
    setPendingDelete({
      id: target,
      name: view?.profile.name || "Unnamed",
    });
  };

  const confirmDelete = async () => {
    if (!pendingDelete) return;
    const { id } = pendingDelete;
    setPendingDelete(null);
    try {
      await deleteTunnel(id);
      setDetailsOpen(false);
      if (selectedId === id) setSelectedId(null);
      await refresh();
    } catch (e: any) {
      setError(String(e?.message ?? e));
    }
  };

  const handleToggle = async (patch: Partial<Profile>) => {
    if (!selected) return;
    await saveTunnel({ ...selected.profile, ...patch });
    await refresh();
    if (selectedId) void setSelection(selectedId);
  };

  const handlePin = async (id?: string, pinned?: boolean) => {
    const target = id ?? selectedId;
    if (!target) return;
    const view = tunnels.find((t) => t.profile.id === target);
    if (!view) return;
    const next = pinned ?? !view.profile.pinned;
    try {
      await saveTunnel({ ...view.profile, pinned: next });
      await refresh();
      void setSelection(target);
    } catch (e: any) {
      setError(String(e?.message ?? e));
    }
  };

  const handleTagsChange = async (tags: string[]) => {
    if (!selected) return;
    await saveTunnel({ ...selected.profile, tags });
    await refresh();
  };

  const handleConnect = async (id?: string) => {
    const target = id ?? selectedId;
    if (!target) return;
    try {
      await connectTunnel(target);
    } catch (e: any) {
      setError(String(e?.message ?? e));
    }
  };

  const handleDisconnect = async (id?: string) => {
    const target = id ?? selectedId;
    if (!target) return;
    try {
      await disconnectTunnel(target);
    } catch (e: any) {
      setError(String(e?.message ?? e));
    }
  };

  const handleContextAction = async (action: string, tunnelId: string) => {
    setSelectedId(tunnelId);
    const view = tunnels.find((t) => t.profile.id === tunnelId);
    const st = view?.status.Status;
    const connected = st === "connected" || st === "connecting";
    switch (action) {
      case "connect":
        if (!connected) await handleConnect(tunnelId);
        break;
      case "disconnect":
        if (connected) await handleDisconnect(tunnelId);
        break;
      case "pin":
        await handlePin(tunnelId);
        break;
      case "edit":
        startEdit(tunnelId);
        break;
      case "duplicate":
        await handleDuplicate(tunnelId);
        break;
      case "details":
        setDetailsOpen(true);
        break;
      case "delete":
        requestDelete(tunnelId);
        break;
    }
  };

  const menuHandlers = useRef({
    editing,
    selectedId,
    startCreate,
    startEdit,
    handleDuplicate,
    handleConnect,
    handleDisconnect,
    handlePin,
  });
  menuHandlers.current = {
    editing,
    selectedId,
    startCreate,
    startEdit,
    handleDuplicate,
    handleConnect,
    handleDisconnect,
    handlePin,
  };

  useEffect(() => {
    const off = Events.On("menu:command", (ev: any) => {
      const data = (ev?.data ?? ev) as { command?: string; visible?: boolean };
      const cmd = data?.command;
      if (!cmd) return;
      const h = menuHandlers.current;
      switch (cmd) {
        case "newTunnel":
          void h.startCreate();
          break;
        case "newTag":
          if (!h.editing && h.selectedId) setTagOpenSignal((n) => n + 1);
          break;
        case "showInspector":
          if (!h.editing && h.selectedId) setDetailsOpen(true);
          break;
        case "editTunnel":
          h.startEdit();
          break;
        case "duplicateTunnel":
          void h.handleDuplicate();
          break;
        case "connect":
          void h.handleConnect();
          break;
        case "disconnect":
          void h.handleDisconnect();
          break;
        case "setSidebar":
          setSidebarVisible(!!data.visible);
          break;
        case "setPinned":
          void h.handlePin(undefined, !!(data as { pinned?: boolean }).pinned);
          break;
      }
    });
    return () => {
      off?.();
    };
  }, []);

  const passwordModal = passwordPrompt ? (
    <PasswordModal
      prompt={passwordPrompt}
      onSubmit={async (secret, save) => {
        await submitPassword(passwordPrompt.requestId, secret, save);
        setPasswordPrompt(null);
      }}
      onCancel={async () => {
        await submitPassword(passwordPrompt.requestId, "", false);
        setPasswordPrompt(null);
      }}
    />
  ) : null;

  const deleteModal = pendingDelete ? (
    <ConfirmDialog
      title="Delete Tunnel"
      message={`Are you sure you want to delete “${pendingDelete.name}”? This cannot be undone.`}
      confirmLabel="Delete"
      danger
      onConfirm={confirmDelete}
      onCancel={() => setPendingDelete(null)}
    />
  ) : null;

  const sidebar = (
    <Sidebar
      tunnels={visibleTunnels}
      allTunnels={tunnels}
      selectedId={selectedId}
      tagFilter={tagFilter}
      blurred={editing}
      onSelect={(id) => {
        if (editing) return;
        setSelectedId(id);
        setDetailsOpen(false);
      }}
      onAdd={startCreate}
      onOptions={() => {
        void openSettings();
      }}
      onTagFilter={setTagFilter}
      onContextAction={handleContextAction}
    />
  );

  const gridCols = sidebarVisible ? `${sidebarWidth}px 5px 1fr` : "1fr";

  if (editing) {
    return (
      <div className="app" style={{ gridTemplateColumns: gridCols }}>
        {sidebarVisible && (
          <>
            {sidebar}
            <div
              className="sidebar-resize"
              role="separator"
              aria-orientation="vertical"
              aria-label="Resize tunnels sidebar"
              onMouseDown={onSidebarResizeStart}
            />
          </>
        )}
        <main className="main">
          <TunnelEditor
            initial={mode.draft}
            mode={mode.kind}
            onCancel={() => setMode({ kind: "browse" })}
            onSave={handleSave}
          />
        </main>
        {passwordModal}
        {deleteModal}
      </div>
    );
  }

  return (
    <div className="app" style={{ gridTemplateColumns: gridCols }}>
      {sidebarVisible && (
        <>
          {sidebar}
          <div
            className="sidebar-resize"
            role="separator"
            aria-orientation="vertical"
            aria-label="Resize tunnels sidebar"
            onMouseDown={onSidebarResizeStart}
          />
        </>
      )}
      <main className="main">
        {selected ? (
          <TunnelDetail
            view={selected}
            allTags={allTags}
            search={search}
            tagOpenSignal={tagOpenSignal}
            onSearch={setSearch}
            onConnect={() => handleConnect()}
            onDisconnect={() => handleDisconnect()}
            onEdit={() => startEdit()}
            onDuplicate={() => handleDuplicate()}
            onDelete={() => requestDelete()}
            onDetails={() => setDetailsOpen(true)}
            onToggle={handleToggle}
            onTagsChange={handleTagsChange}
          />
        ) : (
          <div className="empty-state">
            <div>
              <div style={{ marginBottom: 12 }}>No tunnels yet</div>
              <button className="btn btn-primary" onClick={startCreate}>
                Create Tunnel
              </button>
              {error && <div className="err-text">{error}</div>}
            </div>
          </div>
        )}
      </main>
      {detailsOpen && selected && (
        <DetailsModal view={selected} onClose={() => setDetailsOpen(false)} />
      )}
      {passwordModal}
      {deleteModal}
    </div>
  );
}
