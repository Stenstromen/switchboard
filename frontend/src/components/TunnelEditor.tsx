import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { ADVANCED_OPTIONS, findOption, groupOptions } from "../advancedOptions";
import type { Forward, Profile, TunnelType } from "../types";
import { newForward } from "../types";
import { ChevronDownIcon, ChevronRightIcon, ForwardTypeIcon, PlusIcon } from "./Icons";

type Tab = "general" | "connection" | "advanced";

const HELP_MIN = 160;
const HELP_MAX = 480;
const HELP_DEFAULT = 240;

type Props = {
  initial: Profile;
  mode: "create" | "edit";
  onCancel: () => void;
  onSave: (p: Profile) => void;
};

export function TunnelEditor({ initial, mode, onCancel, onSave }: Props) {
  const [draft, setDraft] = useState<Profile>({ ...initial, advanced: { ...(initial.advanced ?? {}) }, forwards: [...(initial.forwards ?? [])] });
  const [tab, setTab] = useState<Tab>("general");
  const [advFilter, setAdvFilter] = useState<"all" | "customized">("all");
  const [advQuery, setAdvQuery] = useState("");
  const [selectedAdv, setSelectedAdv] = useState<string | null>(null);
  const [collapsedGroups, setCollapsedGroups] = useState<Record<string, boolean>>({});
  const [helpWidth, setHelpWidth] = useState(HELP_DEFAULT);
  const dragRef = useRef<{ startX: number; startW: number } | null>(null);

  const title = `${mode === "create" ? "New Tunnel" : "Edit Tunnel"} — ${draft.name || "Unnamed"}`;

  const patch = (partial: Partial<Profile>) => setDraft((d) => ({ ...d, ...partial }));

  const setForward = (id: string, partial: Partial<Forward>) => {
    setDraft((d) => ({
      ...d,
      forwards: d.forwards.map((f) => (f.id === id ? { ...f, ...partial } : f)),
    }));
  };

  const addForward = (type: TunnelType) => {
    setDraft((d) => ({ ...d, forwards: [...d.forwards, newForward(type)] }));
  };

  const removeForward = (id: string) => {
    setDraft((d) => ({ ...d, forwards: d.forwards.filter((f) => f.id !== id) }));
  };

  const setAdvanced = (key: string, value: string) => {
    setDraft((d) => {
      const advanced = { ...(d.advanced ?? {}) };
      const def = ADVANCED_OPTIONS.find((o) => o.key === key)?.defaultValue ?? "";
      if (value === def || value === "") delete advanced[key];
      else advanced[key] = value;
      // Mirror a few keys into first-class fields
      const next: Profile = { ...d, advanced };
      if (key === "ServerAliveInterval") next.serverAliveInterval = Number(value) || 0;
      if (key === "ServerAliveCountMax") next.serverAliveCountMax = Number(value) || 0;
      if (key === "Compression") next.compression = value === "yes";
      if (key === "AddressFamily") next.addressFamily = value;
      if (key === "BindAddress") next.bindAddress = value;
      if (key === "StrictHostKeyChecking") next.strictHostKeyChecking = value;
      if (key === "HashKnownHosts") next.hashKnownHosts = value === "yes";
      if (key === "LogLevel") next.logLevel = value;
      return next;
    });
  };

  const advValue = (key: string, fallback: string) => {
    if (draft.advanced?.[key] != null) return draft.advanced[key];
    if (key === "ServerAliveInterval") return String(draft.serverAliveInterval || fallback);
    if (key === "ServerAliveCountMax") return String(draft.serverAliveCountMax || fallback);
    if (key === "Compression") return draft.compression ? "yes" : "no";
    if (key === "AddressFamily") return draft.addressFamily || fallback;
    if (key === "BindAddress") return draft.bindAddress || fallback;
    if (key === "StrictHostKeyChecking") return draft.strictHostKeyChecking || fallback;
    if (key === "HashKnownHosts") return draft.hashKnownHosts ? "yes" : "no";
    if (key === "LogLevel") return draft.logLevel || fallback;
    return fallback;
  };

  const isCustom = (key: string, def: string) => {
    const v = advValue(key, def);
    return v !== def;
  };

  const filtered = useMemo(() => {
    return ADVANCED_OPTIONS.filter((o) => {
      if (advQuery && !o.key.toLowerCase().includes(advQuery.toLowerCase())) return false;
      if (advFilter === "customized" && !isCustom(o.key, o.defaultValue)) return false;
      return true;
    });
  }, [advFilter, advQuery, draft]);

  const grouped = groupOptions(filtered);

  const toggleGroup = (group: string) => {
    setCollapsedGroups((prev) => ({ ...prev, [group]: !prev[group] }));
  };

  const onHelpResizeStart = useCallback((e: React.MouseEvent) => {
    e.preventDefault();
    dragRef.current = { startX: e.clientX, startW: helpWidth };
    const onMove = (ev: MouseEvent) => {
      const drag = dragRef.current;
      if (!drag) return;
      // Dragging the handle left grows help; right shrinks it.
      const next = Math.min(HELP_MAX, Math.max(HELP_MIN, drag.startW - (ev.clientX - drag.startX)));
      setHelpWidth(next);
    };
    const onUp = () => {
      dragRef.current = null;
      window.removeEventListener("mousemove", onMove);
      window.removeEventListener("mouseup", onUp);
      document.body.classList.remove("is-resizing-col");
    };
    document.body.classList.add("is-resizing-col");
    window.addEventListener("mousemove", onMove);
    window.addEventListener("mouseup", onUp);
  }, [helpWidth]);

  useEffect(() => {
    return () => document.body.classList.remove("is-resizing-col");
  }, []);

  const hostPlaceholder = draft.port && draft.port !== 22
    ? `Hostname or IP address : ${draft.port}`
    : "Hostname or IP address : 22";

  return (
    <div className="editor">
      <div className="editor-header">
        <div className="editor-title">{title}</div>
        <div className="tabs">
          {(["general", "connection", "advanced"] as Tab[]).map((t) => (
            <button key={t} className={`tab${tab === t ? " is-active" : ""}`} onClick={() => setTab(t)}>
              {t[0].toUpperCase() + t.slice(1)}
            </button>
          ))}
        </div>
      </div>

      <div className="editor-body scrollable">
        {tab === "general" && (
          <>
            <div className="field">
              <div className="field-label">Descriptive name</div>
              <input value={draft.name} onChange={(e) => patch({ name: e.target.value })} />
            </div>

            <div className="login-box">
              <div className="login-row">
                <span>Host</span>
                <input
                  placeholder={hostPlaceholder}
                  value={draft.hostName + (draft.port && draft.port !== 22 ? ` : ${draft.port}` : "")}
                  onChange={(e) => {
                    const raw = e.target.value;
                    const m = raw.match(/^(.*?)\s*:\s*(\d+)\s*$/);
                    if (m) patch({ hostName: m[1].trim(), port: Number(m[2]) || 22 });
                    else patch({ hostName: raw.trim() });
                  }}
                />
              </div>
              <div className="login-row">
                <span>Username</span>
                <input value={draft.user} onChange={(e) => patch({ user: e.target.value })} placeholder="Username" />
              </div>
            </div>

            <ForwardEditor
              title="Dynamic Port Forwarding"
              type="dynamic"
              forwards={draft.forwards.filter((f) => f.type === "dynamic")}
              onAdd={() => addForward("dynamic")}
              onRemove={removeForward}
              onChange={setForward}
            />
            <ForwardEditor
              title="Local Port Forwarding"
              type="local"
              forwards={draft.forwards.filter((f) => f.type === "local")}
              onAdd={() => addForward("local")}
              onRemove={removeForward}
              onChange={setForward}
            />
            <ForwardEditor
              title="Remote Port Forwarding"
              type="remote"
              forwards={draft.forwards.filter((f) => f.type === "remote")}
              onAdd={() => addForward("remote")}
              onRemove={removeForward}
              onChange={setForward}
            />
            <ForwardEditor
              title="Reverse Dynamic Port Forwarding"
              type="reverse-dynamic"
              forwards={draft.forwards.filter((f) => f.type === "reverse-dynamic")}
              onAdd={() => addForward("reverse-dynamic")}
              onRemove={removeForward}
              onChange={setForward}
            />
          </>
        )}

        {tab === "connection" && (
          <>
            <div className="setting-row">
              <div>
                <div className="title">Log Level</div>
              </div>
              <div className="segmented">
                {["INFO", "DEBUG1", "DEBUG2", "DEBUG3"].map((lvl) => (
                  <button
                    key={lvl}
                    className={draft.logLevel === lvl ? "is-active" : ""}
                    onClick={() => patch({ logLevel: lvl })}
                  >
                    {lvl}
                  </button>
                ))}
              </div>
            </div>

            <div className="setting-row">
              <div className="title">Retry attempts before stop connecting</div>
              <input
                style={{ width: 80, textAlign: "right" }}
                value={draft.retryAttempts}
                onChange={(e) => patch({ retryAttempts: Number(e.target.value) || 0 })}
              />
            </div>

            <div className="field-stack">
              <div className="label-row">
                <span className="title">Bind Address</span>
                <input
                  style={{ width: 220 }}
                  placeholder="Bind address"
                  value={draft.bindAddress}
                  onChange={(e) => patch({ bindAddress: e.target.value })}
                />
              </div>
              <div className="desc">Only useful on systems with more than one address.</div>
            </div>

            <div className="setting-row">
              <div>
                <div className="title">Address Family</div>
                <div className="desc">Specifies which address family to use when connecting.</div>
              </div>
              <div className="segmented">
                {[
                  ["any", "Any"],
                  ["inet", "IPv4"],
                  ["inet6", "IPv6"],
                ].map(([v, label]) => (
                  <button
                    key={v}
                    className={draft.addressFamily === v ? "is-active" : ""}
                    onClick={() => patch({ addressFamily: v })}
                  >
                    {label}
                  </button>
                ))}
              </div>
            </div>

            <div className="field-stack">
              <div className="label-row">
                <span className="title">Proxy Jump</span>
                <input
                  style={{ width: 280 }}
                  placeholder="Jump hosts"
                  value={draft.proxyJump}
                  onChange={(e) => patch({ proxyJump: e.target.value })}
                />
              </div>
              <div className="desc">
                Connect to the target host by first making a ssh connection to the jump host, multiple jump hops may be
                specified separated by comma characters.
              </div>
            </div>

            <div className="setting-row">
              <div>
                <div className="title">Do not request session on the remote system (recommended)</div>
                <div className="desc">Corresponds to ssh command option -N, this is useful for just forwarding ports.</div>
              </div>
              <button
                className={`toggle${draft.noSession ? " is-on" : ""}`}
                onClick={() => patch({ noSession: !draft.noSession })}
              />
            </div>

            <div className="setting-row">
              <div>
                <div className="title">Requests compression of all data</div>
                <div className="desc">
                  Compression is desirable on modem lines and other slow connections, but will only slow down things on
                  fast networks.
                </div>
              </div>
              <button
                className={`toggle${draft.compression ? " is-on" : ""}`}
                onClick={() => patch({ compression: !draft.compression })}
              />
            </div>

            <div className="identity-head">
              <span>Identity</span>
            </div>

            <div className="setting-row">
              <div className="title">Private Key</div>
              <input
                style={{ width: 280 }}
                placeholder="None"
                value={draft.identityFile}
                onChange={(e) => patch({ identityFile: e.target.value })}
              />
            </div>
            <div className="setting-row">
              <div className="title">Certificate</div>
              <input
                style={{ width: 280 }}
                placeholder="None"
                value={draft.certificate}
                onChange={(e) => patch({ certificate: e.target.value })}
              />
            </div>
            <div className="setting-row">
              <div>
                <div className="title">Enables forwarding of the authentication agent connection</div>
                <div className="desc">
                  Agent forwarding should be enabled with caution — remote hosts can access the local agent through the
                  forwarded connection.
                </div>
              </div>
              <button
                className={`toggle${draft.forwardAgent ? " is-on" : ""}`}
                onClick={() => patch({ forwardAgent: !draft.forwardAgent })}
              />
            </div>
          </>
        )}

        {tab === "advanced" && (
          <>
            <div className="advanced-toolbar">
              <div className="segmented">
                <button className={advFilter === "customized" ? "is-active" : ""} onClick={() => setAdvFilter("customized")}>
                  Customized
                </button>
                <button className={advFilter === "all" ? "is-active" : ""} onClick={() => setAdvFilter("all")}>
                  All
                </button>
              </div>
              <div className="search-field" style={{ marginLeft: "auto" }}>
                <input placeholder="Search" value={advQuery} onChange={(e) => setAdvQuery(e.target.value)} />
              </div>
            </div>

            <div className="advanced-groups" style={{ gridTemplateColumns: `1fr 6px ${helpWidth}px` }}>
              <div className="adv-list scrollable">
                {Object.entries(grouped).map(([group, opts]) => {
                  const collapsed = !!collapsedGroups[group];
                  return (
                    <div key={group} className={`adv-group${collapsed ? " is-collapsed" : ""}`}>
                      <button
                        type="button"
                        className="adv-group-title"
                        onClick={() => toggleGroup(group)}
                        aria-expanded={!collapsed}
                      >
                        <span className="adv-group-chevron" aria-hidden>
                          {collapsed ? <ChevronRightIcon size={15} /> : <ChevronDownIcon size={15} />}
                        </span>
                        <span>{group}</span>
                      </button>
                      {!collapsed &&
                        opts.map((o) => {
                          const value = advValue(o.key, o.defaultValue);
                          const custom = isCustom(o.key, o.defaultValue);
                          return (
                            <div
                              key={o.key}
                              className={`adv-row${custom ? " is-custom" : ""}${selectedAdv === o.key ? " is-selected" : ""}`}
                              onClick={() => setSelectedAdv(o.key)}
                            >
                              <div className="adv-key">{o.key}</div>
                              <div className="adv-val">
                                {o.kind === "enum" ? (
                                  <select
                                    value={value}
                                    onChange={(e) => setAdvanced(o.key, e.target.value)}
                                    onClick={(e) => e.stopPropagation()}
                                  >
                                    {o.options!.map((opt) => (
                                      <option key={opt} value={opt}>
                                        {opt}
                                      </option>
                                    ))}
                                  </select>
                                ) : (
                                  <input
                                    value={value}
                                    onChange={(e) => setAdvanced(o.key, e.target.value)}
                                    onClick={(e) => e.stopPropagation()}
                                  />
                                )}
                              </div>
                            </div>
                          );
                        })}
                    </div>
                  );
                })}
              </div>
              <div
                className="adv-resize"
                role="separator"
                aria-orientation="vertical"
                aria-label="Resize Quick Help"
                onMouseDown={onHelpResizeStart}
              />
              <div className="adv-help">
                {(() => {
                  const opt = findOption(selectedAdv);
                  if (!opt) return <p className="adv-help-body">Select an option to see Quick Help.</p>;
                  const help = opt.help?.trim();
                  return (
                    <div>
                      <strong className="adv-help-title">{opt.key}</strong>
                      <p className="adv-help-body">
                        {help || "No Quick Help text available for this option."}
                      </p>
                    </div>
                  );
                })()}
              </div>
            </div>
          </>
        )}
      </div>

      <div className="footer-bar">
        <div className="left">
          <button className="btn" onClick={onCancel}>
            Cancel
          </button>
        </div>
        <div className="right">
          <button
            className="btn btn-primary"
            onClick={() => onSave(draft)}
            disabled={!draft.hostName.trim()}
          >
            {mode === "create" ? "Create" : "Save"}
          </button>
        </div>
      </div>
    </div>
  );
}

function ForwardEditor({
  title,
  type,
  forwards,
  onAdd,
  onRemove,
  onChange,
}: {
  title: string;
  type: TunnelType;
  forwards: Forward[];
  onAdd: () => void;
  onRemove: (id: string) => void;
  onChange: (id: string, partial: Partial<Forward>) => void;
}) {
  const dynamic = type === "dynamic" || type === "reverse-dynamic";
  return (
    <section className="section">
      <div className="section-header">
        <ForwardTypeIcon type={type} size={18} />
        <span>{title}</span>
        <div className="section-actions">
          <button className="icon-btn" onClick={onAdd} aria-label="Add">
            <PlusIcon size={13} />
          </button>
          <button
            className="icon-btn"
            onClick={() => forwards.length && onRemove(forwards[forwards.length - 1].id)}
            aria-label="Remove"
            disabled={!forwards.length}
          >
            <MinusIcon />
          </button>
        </div>
      </div>
      {forwards.length === 0 ? (
        <div className="forward-empty">Click + to add {title} for this tunnel.</div>
      ) : (
        <div className="forward-table">
          {forwards.map((f) => (
            <div className="forward-editor-row" key={f.id} style={dynamic ? { gridTemplateColumns: "1fr 1fr auto" } : undefined}>
              <input
                placeholder="Bind host"
                value={f.localHost}
                onChange={(e) => onChange(f.id, { localHost: e.target.value })}
              />
              <input
                placeholder="Port"
                value={f.localPort || ""}
                onChange={(e) => onChange(f.id, { localPort: Number(e.target.value) || 0 })}
              />
              {!dynamic && (
                <>
                  <input
                    placeholder="Remote host"
                    value={f.remoteHost}
                    onChange={(e) => onChange(f.id, { remoteHost: e.target.value })}
                  />
                  <input
                    placeholder="Remote port"
                    value={f.remotePort || ""}
                    onChange={(e) => onChange(f.id, { remotePort: Number(e.target.value) || 0 })}
                  />
                </>
              )}
              <button className="icon-btn" onClick={() => onRemove(f.id)} aria-label="Remove forward">
                <MinusIcon />
              </button>
            </div>
          ))}
        </div>
      )}
    </section>
  );
}

function MinusIcon() {
  return (
    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.4" strokeLinecap="round">
      <path d="M5 12h14" />
    </svg>
  );
}
