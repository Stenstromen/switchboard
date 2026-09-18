import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { Dialogs, Window } from "@wailsio/runtime";
import {
  addKnownHost,
  exportConfiguration,
  getNotificationPermission,
  getPreferences,
  getSSHDefaults,
  importConfiguration,
  listKnownHosts,
  openPathInEditor,
  removeKnownHost,
  requestNotificationPermission,
  revealPathInFinder,
  sendTestNotification,
  setPreferences,
  type EnvVar,
  type KnownHostEntry,
  type NotificationPermission,
  type Preferences,
  type ShowAs,
  type SSHDefaults,
} from "../api";

type Tab = "general" | "ssh" | "notifications";
type SSHSub = "knownhosts" | "locations" | "env";

const SETTINGS_MIN_H = 260;
const SETTINGS_MAX_H = 720;
const SETTINGS_WIDTH = 640;

/** Settings content for the dedicated macOS Settings window. */
export function SettingsPage() {
  const pageRef = useRef<HTMLDivElement>(null);
  const [tab, setTab] = useState<Tab>("general");
  const [sshSub, setSSHSub] = useState<SSHSub>("knownhosts");
  const [prefs, setPrefs] = useState<Preferences | null>(null);
  const [defaults, setDefaults] = useState<SSHDefaults | null>(null);
  const [hosts, setHosts] = useState<KnownHostEntry[]>([]);
  const [selectedHost, setSelectedHost] = useState<number | null>(null);
  const [addOpen, setAddOpen] = useState(false);
  const [addLine, setAddLine] = useState("");
  const [perm, setPerm] = useState<NotificationPermission | null>(null);
  const [busy, setBusy] = useState(false);
  const [flash, setFlash] = useState<string | null>(null);

  const refreshPerm = async () => {
    try {
      setPerm(await getNotificationPermission());
    } catch (e: any) {
      setPerm({
        authorized: false,
        available: false,
        message: String(e?.message ?? e),
      });
    }
  };

  const refreshPrefs = async () => {
    try {
      const [p, d] = await Promise.all([getPreferences(), getSSHDefaults()]);
      setPrefs(p);
      setDefaults(d);
    } catch {
      setPrefs({ showAs: "both", openAtLogin: false, authAgent: "", configFile: "", envVars: [] });
    }
  };

  const refreshHosts = async () => {
    try {
      setHosts(await listKnownHosts());
    } catch (e: any) {
      setFlash(String(e?.message ?? e));
      setHosts([]);
    }
  };

  useEffect(() => {
    refreshPrefs();
    refreshPerm();
  }, []);

  useEffect(() => {
    if (tab === "ssh" && sshSub === "knownhosts") {
      refreshHosts();
    }
  }, [tab, sshSub]);

  useLayoutEffect(() => {
    const el = pageRef.current;
    if (!el) return;

    let lastH = 0;
    let raf = 0;

    const fit = () => {
      cancelAnimationFrame(raf);
      raf = requestAnimationFrame(async () => {
        const contentH = Math.ceil(el.getBoundingClientRect().height);
        if (contentH < 1) return;
        const next = Math.min(SETTINGS_MAX_H, Math.max(SETTINGS_MIN_H, contentH));
        if (Math.abs(next - lastH) < 2) return;
        lastH = next;
        try {
          const width = Math.max(SETTINGS_WIDTH, Math.round(await Window.Width()));
          await Window.SetMinSize(560, SETTINGS_MIN_H);
          await Window.SetSize(width, next);
        } catch {
          // Runtime unavailable in browser preview.
        }
      });
    };

    fit();
    const ro = new ResizeObserver(fit);
    ro.observe(el);
    return () => {
      cancelAnimationFrame(raf);
      ro.disconnect();
    };
  }, [tab, sshSub, addOpen, hosts.length, prefs, perm, flash]);

  const updatePrefs = async (patch: Partial<Preferences>) => {
    if (!prefs) return;
    const next = { ...prefs, ...patch };
    setPrefs(next);
    setBusy(true);
    setFlash(null);
    try {
      setPrefs(await setPreferences(next));
    } catch (e: any) {
      setFlash(String(e?.message ?? e));
      await refreshPrefs();
    } finally {
      setBusy(false);
    }
  };

  const enable = async () => {
    setBusy(true);
    setFlash(null);
    try {
      const next = await requestNotificationPermission();
      setPerm(next);
      if (next.authorized) setFlash("Permission granted.");
    } catch (e: any) {
      setFlash(String(e?.message ?? e));
      await refreshPerm();
    } finally {
      setBusy(false);
    }
  };

  const test = async () => {
    setBusy(true);
    setFlash(null);
    try {
      await sendTestNotification();
      setFlash("Test notification sent.");
    } catch (e: any) {
      setFlash(String(e?.message ?? e));
      await refreshPerm();
    } finally {
      setBusy(false);
    }
  };

  const exportConfig = async () => {
    setBusy(true);
    setFlash(null);
    try {
      const path = await Dialogs.SaveFile({
        Title: "Export Switchboard Configuration",
        Filename: "switchboard-config.json",
        CanCreateDirectories: true,
        Filters: [{ DisplayName: "JSON", Pattern: "*.json" }],
      });
      if (!path) return;
      const result = await exportConfiguration(path);
      setFlash(result.message);
      await refreshPrefs();
    } catch (e: any) {
      setFlash(String(e?.message ?? e));
    } finally {
      setBusy(false);
    }
  };

  const importConfig = async (replace: boolean) => {
    setFlash(null);
    if (replace) {
      const answer = await Dialogs.Question({
        Title: "Replace Configuration?",
        Message:
          "This will replace all tunnels with those from the file. Keychain passwords for removed tunnels will be deleted. Continue?",
        Buttons: [
          { Label: "Cancel", IsCancel: true },
          { Label: "Replace", IsDefault: true },
        ],
      });
      if (answer !== "Replace") return;
    }
    setBusy(true);
    try {
      const path = await Dialogs.OpenFile({
        Title: replace ? "Replace Switchboard Configuration" : "Import Switchboard Configuration",
        CanChooseFiles: true,
        CanChooseDirectories: false,
        AllowsMultipleSelection: false,
        Filters: [{ DisplayName: "JSON", Pattern: "*.json" }],
      });
      if (!path) return;
      const result = await importConfiguration(path, replace);
      setFlash(result.message);
      await refreshPrefs();
    } catch (e: any) {
      setFlash(String(e?.message ?? e));
    } finally {
      setBusy(false);
    }
  };

  const title =
    tab === "general" ? "General" : tab === "ssh" ? "SSH" : "Notifications";
  const showAs: ShowAs = prefs?.showAs ?? "both";

  return (
    <div className={`settings-page${addOpen ? " is-sheet-open" : ""}`} ref={pageRef}>
      <div className="settings-titlebar">
        <span className="settings-title">{title}</span>
      </div>

      <div className="settings-toolbar">
        <button
          className={`settings-tab${tab === "general" ? " is-active" : ""}`}
          onClick={() => setTab("general")}
        >
          <span className="settings-tab-icon" aria-hidden>
            <GearIcon />
          </span>
          <span>General</span>
        </button>
        <button
          className={`settings-tab${tab === "ssh" ? " is-active" : ""}`}
          onClick={() => setTab("ssh")}
        >
          <span className="settings-tab-icon" aria-hidden>
            <ShieldIcon />
          </span>
          <span>SSH</span>
        </button>
        <button
          className={`settings-tab${tab === "notifications" ? " is-active" : ""}`}
          onClick={() => setTab("notifications")}
        >
          <span className="settings-tab-icon" aria-hidden>
            <BellIcon />
          </span>
          <span>Notifications</span>
        </button>
      </div>

      <div className="settings-body scrollable">
        {tab === "general" && (
          <div className="settings-section">
            <div className="settings-pref-row">
              <div className="settings-pref-label">Show as:</div>
              <div className="settings-radio-group is-horizontal" role="radiogroup" aria-label="Show as">
                {(
                  [
                    { value: "dock", label: "Dock icon" },
                    { value: "menubar", label: "Menu bar icon" },
                    { value: "both", label: "Both" },
                  ] as const
                ).map((opt) => (
                  <label key={opt.value} className="settings-radio">
                    <input
                      type="radio"
                      name="showAs"
                      value={opt.value}
                      checked={showAs === opt.value}
                      disabled={busy || prefs == null}
                      onChange={() => updatePrefs({ showAs: opt.value })}
                    />
                    <span>{opt.label}</span>
                  </label>
                ))}
              </div>
            </div>

            <div className="settings-pref-row">
              <div className="settings-pref-label">Open at Login:</div>
              <label className="settings-check">
                <input
                  type="checkbox"
                  checked={!!prefs?.openAtLogin}
                  disabled={busy || prefs == null}
                  onChange={(e) => updatePrefs({ openAtLogin: e.target.checked })}
                />
                <span>Open Switchboard automatically when you log in</span>
              </label>
            </div>

            <div className="settings-divider" />

            <div className="settings-row">
              <div>
                <div className="settings-row-title">Configuration</div>
                <div className="settings-row-desc">
                  Export or import tunnels and preferences as JSON. Passwords stored in Keychain are never included.
                </div>
              </div>
            </div>
            <div className="settings-transfer-actions">
              <button className="btn" disabled={busy} onClick={exportConfig}>
                Export…
              </button>
              <button className="btn" disabled={busy} onClick={() => importConfig(false)}>
                Import & Merge…
              </button>
              <button className="btn" disabled={busy} onClick={() => importConfig(true)}>
                Import & Replace…
              </button>
            </div>

            {flash && <div className="settings-flash is-warn">{flash}</div>}
          </div>
        )}

        {tab === "ssh" && (
          <div className="settings-section">
            <div className="settings-subnav">
              {(
                [
                  { id: "knownhosts", label: "Known Hosts" },
                  { id: "locations", label: "Locations" },
                  { id: "env", label: "Environment Variables" },
                ] as const
              ).map((s) => (
                <button
                  key={s.id}
                  className={`settings-subtab${sshSub === s.id ? " is-active" : ""}`}
                  onClick={() => setSSHSub(s.id)}
                >
                  {s.label}
                </button>
              ))}
            </div>

            {sshSub === "knownhosts" && (
              <KnownHostsPanel
                hosts={hosts}
                selected={selectedHost}
                path={defaults?.knownHosts ?? ""}
                busy={busy}
                onSelect={setSelectedHost}
                onAdd={() => {
                  setAddLine("");
                  setAddOpen(true);
                }}
                onRemove={async () => {
                  if (selectedHost == null) return;
                  setBusy(true);
                  try {
                    await removeKnownHost(selectedHost);
                    setSelectedHost(null);
                    await refreshHosts();
                  } catch (e: any) {
                    setFlash(String(e?.message ?? e));
                  } finally {
                    setBusy(false);
                  }
                }}
                onReveal={() => defaults?.knownHosts && revealPathInFinder(defaults.knownHosts)}
              />
            )}

            {sshSub === "locations" && prefs && (
              <LocationsPanel
                prefs={prefs}
                defaults={defaults}
                busy={busy}
                onChange={updatePrefs}
              />
            )}

            {sshSub === "env" && prefs && (
              <EnvVarsPanel
                vars={prefs.envVars}
                busy={busy}
                onChange={(envVars) => updatePrefs({ envVars })}
              />
            )}

            {flash && tab === "ssh" && <div className="settings-flash is-warn">{flash}</div>}
          </div>
        )}

        {tab === "notifications" && (
          <div className="settings-section">
            <div className="settings-row">
              <div>
                <div className="settings-row-title">System permission</div>
                <div className="settings-row-desc">
                  macOS must allow Switchboard to show notifications for connection events.
                </div>
              </div>
              <span className={`settings-badge${perm?.authorized ? " is-ok" : " is-warn"}`}>
                {perm == null ? "Checking…" : perm.authorized ? "Allowed" : "Not allowed"}
              </span>
            </div>

            <p className="settings-status-msg">{perm?.message}</p>

            <div className="settings-actions">
              {!perm?.authorized && (
                <button
                  className="btn btn-primary"
                  disabled={busy || !perm?.available}
                  onClick={enable}
                >
                  Enable Notifications…
                </button>
              )}
              <button className="btn" disabled={busy || !perm?.authorized} onClick={test}>
                Send Test Notification
              </button>
              <button className="btn btn-ghost" disabled={busy} onClick={refreshPerm}>
                Refresh Status
              </button>
            </div>

            {flash && <div className="settings-flash">{flash}</div>}

            <div className="settings-hint">
              If the system dialog doesn’t appear, open{" "}
              <strong>System Settings → Notifications → Switchboard</strong> and turn alerts on.
            </div>
          </div>
        )}
      </div>

      {addOpen && (
        <div className="settings-sheet">
          <div className="settings-sheet-bar">
            <button className="btn btn-ghost" onClick={() => setAddOpen(false)}>
              Cancel
            </button>
            <button
              className="btn btn-primary"
              disabled={busy || !addLine.trim()}
              onClick={async () => {
                setBusy(true);
                try {
                  await addKnownHost(addLine.trim());
                  setAddOpen(false);
                  await refreshHosts();
                } catch (e: any) {
                  setFlash(String(e?.message ?? e));
                } finally {
                  setBusy(false);
                }
              }}
            >
              Add
            </button>
          </div>
          <textarea
            className="settings-sheet-input mono"
            spellCheck={false}
            placeholder="Enter known host entry"
            value={addLine}
            onChange={(e) => setAddLine(e.target.value)}
            autoFocus
          />
        </div>
      )}
    </div>
  );
}

function KnownHostsPanel({
  hosts,
  selected,
  path,
  busy,
  onSelect,
  onAdd,
  onRemove,
  onReveal,
}: {
  hosts: KnownHostEntry[];
  selected: number | null;
  path: string;
  busy: boolean;
  onSelect: (i: number) => void;
  onAdd: () => void;
  onRemove: () => void;
  onReveal: () => void;
}) {
  return (
    <>
      <p className="settings-blurb">
        The known_hosts file stores the public keys of remote servers you’ve connected to, helping
        to ensure that you’re connecting to the same server each time and preventing
        man-in-the-middle attacks.
      </p>
      <div className="settings-list-box">
        <div className="settings-list scrollable">
          {hosts.length === 0 ? (
            <div className="settings-list-empty">No known hosts yet</div>
          ) : (
            hosts.map((h) => (
              <button
                key={h.index}
                className={`settings-list-item${selected === h.index ? " is-selected" : ""}`}
                onClick={() => onSelect(h.index)}
              >
                <div className="settings-list-title mono">
                  {h.hosts} {h.keyType}
                </div>
                <div className="settings-list-sub mono">{h.keyData}</div>
              </button>
            ))
          )}
        </div>
        <div className="settings-list-actions">
          <button className="icon-btn" onClick={onAdd} disabled={busy} title="Add" aria-label="Add">
            +
          </button>
          <button
            className="icon-btn"
            onClick={onRemove}
            disabled={busy || selected == null}
            title="Remove"
            aria-label="Remove"
          >
            −
          </button>
        </div>
      </div>
      <div className="settings-location-footer">
        <span>
          Location: <span className="mono">{path || "—"}</span>
        </span>
        <button
          className="icon-btn"
          disabled={!path}
          onClick={onReveal}
          title="Reveal in Finder"
          aria-label="Reveal in Finder"
        >
          ›
        </button>
      </div>
    </>
  );
}

function LocationsPanel({
  prefs,
  defaults,
  busy,
  onChange,
}: {
  prefs: Preferences;
  defaults: SSHDefaults | null;
  busy: boolean;
  onChange: (patch: Partial<Preferences>) => void;
}) {
  const defaultAuth = defaults?.authAgent ?? "";
  const defaultConfig = defaults?.configFile ?? "";
  const [auth, setAuth] = useState(prefs.authAgent || defaultAuth);
  const [config, setConfig] = useState(prefs.configFile || defaultConfig);

  useEffect(() => {
    setAuth(prefs.authAgent || defaultAuth);
    setConfig(prefs.configFile || defaultConfig);
  }, [prefs.authAgent, prefs.configFile, defaultAuth, defaultConfig]);

  const commit = () => {
    const nextAuth = auth.trim() === defaultAuth ? "" : auth.trim();
    const nextConfig = config.trim() === defaultConfig ? "" : config.trim();
    if (nextAuth === prefs.authAgent && nextConfig === prefs.configFile) return;
    onChange({ authAgent: nextAuth, configFile: nextConfig });
  };

  return (
    <>
      <div className="settings-field">
        <label className="settings-field-label">Auth Agent:</label>
        <input
          className="settings-field-input mono"
          spellCheck={false}
          value={auth}
          placeholder={defaultAuth || "SSH_AUTH_SOCK"}
          disabled={busy}
          onChange={(e) => setAuth(e.target.value)}
          onBlur={commit}
        />
        <p className="settings-field-help">
          Identifies the path of a UNIX-domain socket used to communicate with the agent. Overwrite
          this value if you’re using an agent other than ssh-agent, e.g. gpg-agent.
        </p>
      </div>
      <div className="settings-field">
        <label className="settings-field-label">Config File:</label>
        <div className="settings-field-row">
          <input
            className="settings-field-input mono"
            spellCheck={false}
            value={config}
            placeholder={defaultConfig || "~/.ssh/config"}
            disabled={busy}
            onChange={(e) => setConfig(e.target.value)}
            onBlur={commit}
          />
          <button
            className="btn"
            disabled={busy || !(config.trim() || defaultConfig)}
            onClick={() => openPathInEditor(config.trim() || defaultConfig)}
          >
            Edit
          </button>
        </div>
        <p className="settings-field-help">
          The default configuration file is ~/.ssh/config. If you overwrite this value, the
          system-wide configuration file (/etc/ssh/ssh_config) will be ignored.
        </p>
      </div>
    </>
  );
}

function EnvVarsPanel({
  vars,
  busy,
  onChange,
}: {
  vars: EnvVar[];
  busy: boolean;
  onChange: (vars: EnvVar[]) => void;
}) {
  const [local, setLocal] = useState(vars);
  const [selected, setSelected] = useState<number | null>(null);

  useEffect(() => {
    setLocal(vars);
  }, [vars]);

  const commit = (next: EnvVar[]) => {
    setLocal(next);
    onChange(next);
  };

  const commitLocal = () => {
    setLocal((prev) => {
      const same =
        prev.length === vars.length &&
        prev.every(
          (v, i) =>
            v.name === vars[i]?.name &&
            v.value === vars[i]?.value &&
            v.enabled === vars[i]?.enabled
        );
      if (!same) onChange(prev);
      return prev;
    });
  };

  return (
    <>
      <p className="settings-blurb">
        Specify runtime environment variables for ssh or local terminal processes.
      </p>
      <div className="settings-env-table">
        <div className="settings-env-head">
          <span>Name</span>
          <span>Value</span>
          <span>Enabled</span>
        </div>
        <div className="settings-env-body scrollable">
          {local.length === 0 ? (
            <div className="settings-list-empty">No environment variables</div>
          ) : (
            local.map((v, i) => (
              <div
                key={i}
                className={`settings-env-row${selected === i ? " is-selected" : ""}`}
                onClick={() => setSelected(i)}
              >
                <input
                  className="mono"
                  spellCheck={false}
                  value={v.name}
                  disabled={busy}
                  placeholder="NAME"
                  onChange={(e) => {
                    const name = e.target.value;
                    setLocal((prev) => prev.map((row, idx) => (idx === i ? { ...row, name } : row)));
                  }}
                  onBlur={commitLocal}
                />
                <input
                  className="mono"
                  spellCheck={false}
                  value={v.value}
                  disabled={busy}
                  placeholder="value"
                  onChange={(e) => {
                    const value = e.target.value;
                    setLocal((prev) => prev.map((row, idx) => (idx === i ? { ...row, value } : row)));
                  }}
                  onBlur={commitLocal}
                />
                <label className="settings-check" onClick={(e) => e.stopPropagation()}>
                  <input
                    type="checkbox"
                    checked={v.enabled}
                    disabled={busy}
                    onChange={(e) => {
                      const enabled = e.target.checked;
                      setLocal((prev) => {
                        const next = prev.map((row, idx) =>
                          idx === i ? { ...row, enabled } : row
                        );
                        onChange(next);
                        return next;
                      });
                    }}
                  />
                </label>
              </div>
            ))
          )}
        </div>
        <div className="settings-list-actions">
          <button
            className="icon-btn"
            disabled={busy}
            title="Add"
            aria-label="Add"
            onClick={() => {
              const next = [...local, { name: "", value: "", enabled: true }];
              commit(next);
              setSelected(next.length - 1);
            }}
          >
            +
          </button>
          <button
            className="icon-btn"
            disabled={busy || selected == null}
            title="Remove"
            aria-label="Remove"
            onClick={() => {
              if (selected == null) return;
              commit(local.filter((_, i) => i !== selected));
              setSelected(null);
            }}
          >
            −
          </button>
        </div>
      </div>
    </>
  );
}

function GearIcon() {
  return (
    <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7">
      <circle cx="12" cy="12" r="3" />
      <path d="M12 1.5v3M12 19.5v3M4.9 4.9l2.1 2.1M17 17l2.1 2.1M1.5 12h3M19.5 12h3M4.9 19.1L7 17M17 7l2.1-2.1" />
    </svg>
  );
}

function BellIcon() {
  return (
    <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7">
      <path d="M6 9a6 6 0 1 1 12 0c0 7 3 7 3 7H3s3 0 3-7" />
      <path d="M10 19a2 2 0 0 0 4 0" />
    </svg>
  );
}

function ShieldIcon() {
  return (
    <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7">
      <path d="M12 3l7 3v5c0 5-3.5 8.5-7 10-3.5-1.5-7-5-7-10V6l7-3z" />
      <path d="M9 12h6M9 9h6M9 15h4" />
    </svg>
  );
}
