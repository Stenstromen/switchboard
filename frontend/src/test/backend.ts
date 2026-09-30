import { act } from "@testing-library/react";
import { vi } from "vitest";
import type { Forward, Profile, Snapshot, Status, TunnelView } from "../types";

type Handler = (ev: { data: unknown }) => void;

export type SeedInput = {
  id?: string;
  name: string;
  hostName?: string;
  user?: string;
  port?: number;
  tags?: string[];
  pinned?: boolean;
  autoConnect?: boolean;
  notifyOnStatusChange?: boolean;
  forwards?: Forward[];
  proxyJump?: string;
  logLevel?: string;
  status?: Status;
  err?: string;
  logs?: string;
  pid?: number;
};

type StoredProfile = Profile;
type StoredView = { profile: StoredProfile; status: Snapshot };

const backend = vi.hoisted(() => {
  const listeners = new Map<string, Set<Handler>>();
  const store = {
    seq: 1,
    listError: null as string | null,
    tunnels: [] as StoredView[],
    prefs: {
      showAs: "both" as "dock" | "menubar" | "both",
      openAtLogin: false,
      authAgent: "",
      configFile: "/Users/me/.ssh/config",
      envVars: [] as Array<{ name: string; value: string; enabled: boolean }>,
    },
    defaults: {
      authAgent: "SSH_AUTH_SOCK",
      configFile: "/Users/me/.ssh/config",
      knownHosts: "/Users/me/.ssh/known_hosts",
    },
    hosts: [] as Array<{ index: number; line: string; hosts: string; keyType: string; keyData: string }>,
    perm: {
      authorized: false,
      available: true,
      message: "Notifications are turned off.",
    },
  };

  const clone = <T,>(value: T): T => structuredClone(value);

  const blank = (id: string, over: Partial<Profile> = {}): Profile => ({
    id,
    name: "Unnamed",
    hostName: "",
    user: "",
    port: 22,
    proxyJump: "",
    identityFile: "",
    certificate: "",
    strictHostKeyChecking: "",
    hashKnownHosts: false,
    serverAliveInterval: 0,
    serverAliveCountMax: 0,
    logLevel: "INFO",
    retryAttempts: 999,
    bindAddress: "",
    addressFamily: "any",
    noSession: true,
    compression: false,
    forwardAgent: false,
    notifyOnStatusChange: true,
    autoConnect: false,
    pinned: false,
    ...over,
    advanced: { ...(over.advanced ?? {}) },
    forwards: [...(over.forwards ?? [])],
    tags: [...(over.tags ?? [])],
  });

  const emitNow = (name: string, data: unknown) => {
    for (const handler of listeners.get(name) ?? []) handler({ data });
  };

  const must = (id: string) => {
    const view = store.tunnels.find((t) => t.profile.id === id);
    if (!view) throw new Error(`unknown tunnel ${id}`);
    return view;
  };

  const dialogs = {
    SaveFile: vi.fn(async () => ""),
    OpenFile: vi.fn(async () => ""),
    Question: vi.fn(async () => "Cancel"),
  };
  const windowApi = {
    Width: vi.fn(async () => 640),
    SetMinSize: vi.fn(async () => undefined),
    SetSize: vi.fn(async () => undefined),
  };

  const listTunnels = vi.fn(async () => {
    if (store.listError) throw new Error(store.listError);
    return clone(store.tunnels);
  });
  const newTunnelDraft = vi.fn(async () => blank(`draft-${store.seq++}`));
  const saveTunnel = vi.fn(async (p: Profile) => {
    const profile = blank(p.id || `tun-${store.seq++}`, p);
    const existing = store.tunnels.find((t) => t.profile.id === profile.id);
    const view: StoredView = {
      profile,
      status: existing?.status ?? { Status: "disconnected", Err: "", PID: 0, Logs: "" },
    };
    if (existing) store.tunnels[store.tunnels.indexOf(existing)] = view;
    else store.tunnels.push(view);
    return clone(view);
  });
  const deleteTunnel = vi.fn(async (id: string) => {
    store.tunnels = store.tunnels.filter((t) => t.profile.id !== id);
  });
  const connectTunnel = vi.fn(async (id: string) => {
    const view = must(id);
    view.status = { Status: "connected", Err: "", PID: 4321, Logs: "ready\n" };
    emitNow("tunnel:status", { hostId: id, snap: clone(view.status) });
  });
  const disconnectTunnel = vi.fn(async (id: string) => {
    const view = must(id);
    view.status = { Status: "disconnected", Err: "", PID: 0, Logs: "" };
    emitNow("tunnel:status", { hostId: id, snap: clone(view.status) });
  });
  const submitPassword = vi.fn(async () => undefined);
  const openSettings = vi.fn(async () => undefined);
  const setSelection = vi.fn(async () => undefined);
  const previewArgs = vi.fn(async (p: Profile) => ["-N", `${p.user}@${p.hostName}`]);
  const getPreferences = vi.fn(async () => clone(store.prefs));
  const setPreferences = vi.fn(async (prefs: typeof store.prefs) => {
    store.prefs = clone(prefs);
    return clone(store.prefs);
  });
  const getSSHDefaults = vi.fn(async () => clone(store.defaults));
  const exportConfiguration = vi.fn(async (path: string) => ({
    path,
    profileCount: store.tunnels.length,
    message: `Exported ${store.tunnels.length} tunnels.`,
  }));
  const importConfiguration = vi.fn(async (path: string, replace: boolean) => ({
    path,
    profileCount: 1,
    added: replace ? 0 : 1,
    updated: 0,
    removed: replace ? store.tunnels.length : 0,
    message: replace ? "Replaced configuration." : "Merged configuration.",
  }));
  const listKnownHosts = vi.fn(async () => clone(store.hosts));
  const addKnownHost = vi.fn(async (line: string) => {
    const [hosts, keyType, ...rest] = line.split(/\s+/);
    store.hosts.push({
      index: store.hosts.length,
      line,
      hosts: hosts ?? line,
      keyType: keyType ?? "",
      keyData: rest.join(" "),
    });
  });
  const removeKnownHost = vi.fn(async (index: number) => {
    store.hosts = store.hosts.filter((h) => h.index !== index);
  });
  const revealPathInFinder = vi.fn(async () => undefined);
  const openPathInEditor = vi.fn(async () => undefined);
  const getNotificationPermission = vi.fn(async () => clone(store.perm));
  const requestNotificationPermission = vi.fn(async () => {
    store.perm = { authorized: true, available: true, message: "Notifications are allowed." };
    return clone(store.perm);
  });
  const sendTestNotification = vi.fn(async () => undefined);

  const fns = [
    listTunnels,
    newTunnelDraft,
    saveTunnel,
    deleteTunnel,
    connectTunnel,
    disconnectTunnel,
    submitPassword,
    openSettings,
    setSelection,
    previewArgs,
    getPreferences,
    setPreferences,
    getSSHDefaults,
    exportConfiguration,
    importConfiguration,
    listKnownHosts,
    addKnownHost,
    removeKnownHost,
    revealPathInFinder,
    openPathInEditor,
    getNotificationPermission,
    requestNotificationPermission,
    sendTestNotification,
  ];

  return {
    store,
    dialogs,
    windowApi,
    on(name: string, handler: Handler) {
      let set = listeners.get(name);
      if (!set) {
        set = new Set();
        listeners.set(name, set);
      }
      set.add(handler);
      return () => set!.delete(handler);
    },
    emitNow,
    reset() {
      listeners.clear();
      store.seq = 1;
      store.listError = null;
      store.tunnels = [];
      store.prefs = {
        showAs: "both",
        openAtLogin: false,
        authAgent: "",
        configFile: "/Users/me/.ssh/config",
        envVars: [],
      };
      store.hosts = [];
      store.perm = {
        authorized: false,
        available: true,
        message: "Notifications are turned off.",
      };
      dialogs.SaveFile.mockReset();
      dialogs.OpenFile.mockReset();
      dialogs.Question.mockReset();
      dialogs.SaveFile.mockResolvedValue("");
      dialogs.OpenFile.mockResolvedValue("");
      dialogs.Question.mockResolvedValue("Cancel");
      windowApi.Width.mockClear();
      windowApi.SetMinSize.mockClear();
      windowApi.SetSize.mockClear();
      for (const fn of fns) fn.mockClear();
    },
    listTunnels,
    newTunnelDraft,
    saveTunnel,
    deleteTunnel,
    connectTunnel,
    disconnectTunnel,
    submitPassword,
    openSettings,
    setSelection,
    previewArgs,
    getPreferences,
    setPreferences,
    getSSHDefaults,
    exportConfiguration,
    importConfiguration,
    listKnownHosts,
    addKnownHost,
    removeKnownHost,
    revealPathInFinder,
    openPathInEditor,
    getNotificationPermission,
    requestNotificationPermission,
    sendTestNotification,
  };
});

const {
  store,
  dialogs,
  emitNow,
  reset,
  listTunnels,
  saveTunnel,
  deleteTunnel,
  connectTunnel,
  disconnectTunnel,
  submitPassword,
  openSettings,
  previewArgs,
  setPreferences,
  exportConfiguration,
  importConfiguration,
  addKnownHost,
  removeKnownHost,
  requestNotificationPermission,
  sendTestNotification,
} = backend;

vi.mock("../api", () => ({
  listTunnels: backend.listTunnels,
  newTunnelDraft: backend.newTunnelDraft,
  saveTunnel: backend.saveTunnel,
  deleteTunnel: backend.deleteTunnel,
  connectTunnel: backend.connectTunnel,
  disconnectTunnel: backend.disconnectTunnel,
  submitPassword: backend.submitPassword,
  openSettings: backend.openSettings,
  setSelection: backend.setSelection,
  previewArgs: backend.previewArgs,
  getPreferences: backend.getPreferences,
  setPreferences: backend.setPreferences,
  getSSHDefaults: backend.getSSHDefaults,
  exportConfiguration: backend.exportConfiguration,
  importConfiguration: backend.importConfiguration,
  listKnownHosts: backend.listKnownHosts,
  addKnownHost: backend.addKnownHost,
  removeKnownHost: backend.removeKnownHost,
  revealPathInFinder: backend.revealPathInFinder,
  openPathInEditor: backend.openPathInEditor,
  getNotificationPermission: backend.getNotificationPermission,
  requestNotificationPermission: backend.requestNotificationPermission,
  sendTestNotification: backend.sendTestNotification,
}));

vi.mock("@wailsio/runtime", () => ({
  Events: { On: backend.on },
  Dialogs: backend.dialogs,
  Window: backend.windowApi,
}));

export const api = {
  listTunnels,
  saveTunnel,
  deleteTunnel,
  connectTunnel,
  disconnectTunnel,
  submitPassword,
  openSettings,
  previewArgs,
  setPreferences,
  exportConfiguration,
  importConfiguration,
  addKnownHost,
  removeKnownHost,
  requestNotificationPermission,
  sendTestNotification,
};

export const dialogsApi = dialogs;

export function resetBackend() {
  reset();
}

export function failList(message: string) {
  store.listError = message;
}

export function seedTunnel(input: SeedInput): TunnelView {
  const id = input.id ?? `tun-${store.seq++}`;
  const profile: Profile = {
    id,
    name: input.name,
    hostName: input.hostName ?? "example.com",
    user: input.user ?? "root",
    port: input.port ?? 22,
    proxyJump: input.proxyJump ?? "",
    identityFile: "",
    certificate: "",
    strictHostKeyChecking: "",
    hashKnownHosts: false,
    serverAliveInterval: 0,
    serverAliveCountMax: 0,
    logLevel: input.logLevel ?? "INFO",
    retryAttempts: 999,
    bindAddress: "",
    addressFamily: "any",
    noSession: true,
    compression: false,
    forwardAgent: false,
    advanced: {},
    forwards: input.forwards ?? [],
    notifyOnStatusChange: input.notifyOnStatusChange ?? true,
    autoConnect: input.autoConnect ?? false,
    pinned: input.pinned ?? false,
    tags: input.tags ?? [],
  };
  const status: Snapshot = {
    Status: input.status ?? "disconnected",
    Err: input.err ?? "",
    PID: input.pid ?? 0,
    Logs: input.logs ?? "",
  };
  const view = { profile, status };
  store.tunnels.push(structuredClone(view));
  return view;
}

export function tunnelByName(name: string): TunnelView {
  const view = store.tunnels.find((t) => t.profile.name === name);
  if (!view) throw new Error(`no tunnel named ${name}`);
  return view;
}

export function emit(name: string, data: unknown) {
  act(() => {
    emitNow(name, data);
  });
}
