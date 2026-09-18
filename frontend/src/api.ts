import * as AppService from "../bindings/github.com/stenstromen/switchboard/appservice";
import type { Forward, Profile, Snapshot, Status, TunnelView } from "./types";

export async function listTunnels(): Promise<TunnelView[]> {
  const views = await AppService.ListTunnels();
  return (views ?? []).map(normalizeView);
}

export async function newTunnelDraft(): Promise<Profile> {
  return normalizeProfile(await AppService.NewTunnelDraft());
}

export async function saveTunnel(p: Profile): Promise<TunnelView> {
  return normalizeView(await AppService.SaveTunnel(denormalizeProfile(p) as any));
}

export async function deleteTunnel(id: string): Promise<void> {
  await AppService.DeleteTunnel(id);
}

export async function connectTunnel(id: string): Promise<void> {
  await AppService.Connect(id);
}

export async function disconnectTunnel(id: string): Promise<void> {
  await AppService.Disconnect(id);
}

export async function submitPassword(requestID: string, secret: string, save = true): Promise<void> {
  await AppService.SubmitPassword(requestID, secret, save);
}

export async function previewArgs(p: Profile): Promise<string[]> {
  return (await AppService.PreviewArgs(denormalizeProfile(p) as any)) ?? [];
}

export type NotificationPermission = {
  authorized: boolean;
  available: boolean;
  message: string;
};

export async function getNotificationPermission(): Promise<NotificationPermission> {
  const p = await AppService.GetNotificationPermission();
  return {
    authorized: !!(p as any).authorized,
    available: !!(p as any).available,
    message: String((p as any).message ?? ""),
  };
}

export async function requestNotificationPermission(): Promise<NotificationPermission> {
  const p = await AppService.RequestNotificationPermission();
  return {
    authorized: !!(p as any).authorized,
    available: !!(p as any).available,
    message: String((p as any).message ?? ""),
  };
}

export async function sendTestNotification(): Promise<void> {
  await AppService.SendTestNotification();
}

export async function openSettings(): Promise<void> {
  await AppService.OpenSettings();
}

export async function openMainWindow(): Promise<void> {
  await AppService.OpenMainWindow();
}

export async function setSelection(id: string): Promise<void> {
  await AppService.SetSelection(id);
}

export async function connectAllTunnels(): Promise<void> {
  await AppService.ConnectAll();
}

export async function disconnectAllTunnels(): Promise<void> {
  await AppService.DisconnectAll();
}

export type ShowAs = "dock" | "menubar" | "both";

export type EnvVar = {
  name: string;
  value: string;
  enabled: boolean;
};

export type Preferences = {
  showAs: ShowAs;
  openAtLogin: boolean;
  authAgent: string;
  configFile: string;
  envVars: EnvVar[];
};

export type SSHDefaults = {
  authAgent: string;
  configFile: string;
  knownHosts: string;
};

export type KnownHostEntry = {
  index: number;
  line: string;
  hosts: string;
  keyType: string;
  keyData: string;
};

export async function getPreferences(): Promise<Preferences> {
  const p = await AppService.GetPreferences();
  return normalizePreferences(p);
}

export async function setPreferences(prefs: Preferences): Promise<Preferences> {
  const p = await AppService.SetPreferences({
    showAs: prefs.showAs,
    openAtLogin: prefs.openAtLogin,
    authAgent: prefs.authAgent,
    configFile: prefs.configFile,
    envVars: prefs.envVars,
  } as any);
  return normalizePreferences(p);
}

export type ConfigTransferResult = {
  path: string;
  profileCount: number;
  added?: number;
  updated?: number;
  removed?: number;
  message: string;
};

export async function exportConfiguration(path: string): Promise<ConfigTransferResult> {
  const r = await AppService.ExportConfiguration(path);
  return {
    path: String((r as any).path ?? path),
    profileCount: Number((r as any).profileCount ?? 0),
    message: String((r as any).message ?? "Exported."),
  };
}

export async function importConfiguration(path: string, replace: boolean): Promise<ConfigTransferResult> {
  const r = await AppService.ImportConfiguration(path, replace);
  return {
    path: String((r as any).path ?? path),
    profileCount: Number((r as any).profileCount ?? 0),
    added: Number((r as any).added ?? 0),
    updated: Number((r as any).updated ?? 0),
    removed: Number((r as any).removed ?? 0),
    message: String((r as any).message ?? "Imported."),
  };
}

export async function getSSHDefaults(): Promise<SSHDefaults> {
  const d = await AppService.GetSSHDefaults();
  return {
    authAgent: String((d as any).authAgent ?? ""),
    configFile: String((d as any).configFile ?? ""),
    knownHosts: String((d as any).knownHosts ?? ""),
  };
}

export async function listKnownHosts(): Promise<KnownHostEntry[]> {
  const list = await AppService.ListKnownHosts();
  return (list ?? []).map((h: any, i: number) => ({
    index: h.index ?? i,
    line: String(h.line ?? ""),
    hosts: String(h.hosts ?? ""),
    keyType: String(h.keyType ?? ""),
    keyData: String(h.keyData ?? ""),
  }));
}

export async function addKnownHost(line: string): Promise<void> {
  await AppService.AddKnownHost(line);
}

export async function removeKnownHost(index: number): Promise<void> {
  await AppService.RemoveKnownHost(index);
}

export async function revealPathInFinder(path: string): Promise<void> {
  await AppService.RevealPathInFinder(path);
}

export async function openPathInEditor(path: string): Promise<void> {
  await AppService.OpenPathInEditor(path);
}

function normalizePreferences(p: any): Preferences {
  const showAs = String(p?.showAs ?? p?.ShowAs ?? "both");
  const envVars = (p?.envVars ?? p?.EnvVars ?? []).map((e: any) => ({
    name: String(e?.name ?? e?.Name ?? ""),
    value: String(e?.value ?? e?.Value ?? ""),
    enabled: !!(e?.enabled ?? e?.Enabled),
  }));
  return {
    showAs: showAs === "dock" || showAs === "menubar" || showAs === "both" ? showAs : "both",
    openAtLogin: !!(p?.openAtLogin ?? p?.OpenAtLogin),
    authAgent: String(p?.authAgent ?? p?.AuthAgent ?? ""),
    configFile: String(p?.configFile ?? p?.ConfigFile ?? ""),
    envVars,
  };
}

function normalizeView(v: any): TunnelView {
  return {
    profile: normalizeProfile(v.profile ?? v.Profile ?? v),
    status: normalizeSnapshot(v.status ?? v.Status),
  };
}

function normalizeSnapshot(s: any): Snapshot {
  if (!s) return { Status: "disconnected", Err: "", PID: 0, Logs: "" };
  return {
    Status: (s.Status ?? s.status ?? "disconnected") as Status,
    Err: s.Err ?? s.err ?? "",
    PID: s.PID ?? s.pid ?? 0,
    Logs: s.Logs ?? s.logs ?? "",
  };
}

function normalizeProfile(p: any): Profile {
  return {
    id: p.id ?? "",
    name: p.name ?? "Unnamed",
    hostName: p.hostName ?? "",
    user: p.user ?? "",
    port: p.port ?? 22,
    proxyJump: p.proxyJump ?? "",
    identityFile: p.identityFile ?? "",
    certificate: p.certificate ?? "",
    strictHostKeyChecking: p.strictHostKeyChecking ?? "",
    hashKnownHosts: !!p.hashKnownHosts,
    serverAliveInterval: p.serverAliveInterval ?? 0,
    serverAliveCountMax: p.serverAliveCountMax ?? 0,
    logLevel: p.logLevel ?? "INFO",
    retryAttempts: p.retryAttempts ?? 999,
    bindAddress: p.bindAddress ?? "",
    addressFamily: p.addressFamily ?? "any",
    noSession: p.noSession !== false,
    compression: !!p.compression,
    forwardAgent: !!p.forwardAgent,
    advanced: p.advanced ?? {},
    forwards: (p.forwards ?? []).map(normalizeForward),
    notifyOnStatusChange: p.notifyOnStatusChange !== false,
    autoConnect: !!p.autoConnect,
    pinned: !!(p?.pinned ?? p?.Pinned),
    tags: p.tags ?? [],
  };
}

function normalizeForward(f: any): Forward {
  return {
    id: f.id || crypto.randomUUID(),
    type: f.type,
    localHost: f.localHost ?? "",
    localPort: f.localPort ?? 0,
    remoteHost: f.remoteHost ?? "",
    remotePort: f.remotePort ?? 0,
  };
}

function denormalizeProfile(p: Profile): Profile {
  return {
    ...p,
    advanced: p.advanced ?? {},
    forwards: p.forwards ?? [],
  };
}
