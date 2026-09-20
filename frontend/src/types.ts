export type TunnelType = "local" | "remote" | "dynamic" | "reverse-dynamic";

export type Status = "disconnected" | "connecting" | "connected" | "error";

export interface Forward {
  id: string;
  type: TunnelType;
  localHost: string;
  localPort: number;
  remoteHost: string;
  remotePort: number;
}

export interface Profile {
  id: string;
  name: string;
  hostName: string;
  user: string;
  port: number;
  proxyJump: string;
  identityFile: string;
  certificate: string;
  strictHostKeyChecking: string;
  hashKnownHosts: boolean;
  serverAliveInterval: number;
  serverAliveCountMax: number;
  logLevel: string;
  retryAttempts: number;
  bindAddress: string;
  addressFamily: string;
  noSession: boolean;
  compression: boolean;
  forwardAgent: boolean;
  advanced?: Record<string, string>;
  forwards: Forward[];
  notifyOnStatusChange: boolean;
  autoConnect: boolean;
  pinned?: boolean;
  tags?: string[];
}

export interface Snapshot {
  Status: Status;
  Err: string;
  PID: number;
  Logs: string;
}

export interface TunnelView {
  profile: Profile;
  status: Snapshot;
}

export interface PasswordPrompt {
  requestId: string;
  hostId: string;
  prompt: string;
  kind: string;
}

export interface StatusEvent {
  hostId: string;
  snap: Snapshot;
}

export function statusLabel(s?: Status): string {
  switch (s) {
    case "connected":
      return "Connected";
    case "connecting":
      return "Connecting";
    case "error":
      return "Error";
    default:
      return "Disconnected";
  }
}

/** Prefer a live error/retry message when the backend provides one. */
export function statusText(snap?: Snapshot): string {
  if (!snap) return statusLabel(undefined);
  const err = (snap.Err || "").trim();
  if (err && (snap.Status === "connecting" || snap.Status === "error")) {
    return err;
  }
  return statusLabel(snap.Status);
}

export function destination(p: Profile): string {
  if (p.user) return `${p.user}@${p.hostName}`;
  return p.hostName || "";
}

export function newForward(type: TunnelType): Forward {
  return {
    id: crypto.randomUUID(),
    type,
    localHost: "127.0.0.1",
    localPort: type === "dynamic" || type === "reverse-dynamic" ? 1080 : 8080,
    remoteHost: type === "dynamic" || type === "reverse-dynamic" ? "" : "127.0.0.1",
    remotePort: type === "dynamic" || type === "reverse-dynamic" ? 0 : 80,
  };
}
