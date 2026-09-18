import { useEffect, useState } from "react";
import { previewArgs } from "../api";
import type { TunnelView } from "../types";
import { destination, statusLabel } from "../types";

type Props = {
  view: TunnelView;
  onClose: () => void;
};

export function DetailsModal({ view, onClose }: Props) {
  const { profile: p, status } = view;
  const [args, setArgs] = useState<string[]>([]);
  const [tab, setTab] = useState<"overview" | "logs" | "command">("overview");

  useEffect(() => {
    let cancelled = false;
    previewArgs(p).then((a) => {
      if (!cancelled) setArgs(a);
    });
    return () => {
      cancelled = true;
    };
  }, [p]);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  return (
    <div className="modal-backdrop" onMouseDown={onClose}>
      <div
        className="modal details-modal"
        role="dialog"
        aria-modal="true"
        onMouseDown={(e) => e.stopPropagation()}
      >
        <div className="details-header">
          <div>
            <h3>{p.name || "Unnamed"}</h3>
            <div className="details-sub">{destination(p)}</div>
          </div>
          <button className="icon-btn" onClick={onClose} aria-label="Close">
            ×
          </button>
        </div>

        <div className="tabs details-tabs">
          {(["overview", "logs", "command"] as const).map((t) => (
            <button key={t} className={`tab${tab === t ? " is-active" : ""}`} onClick={() => setTab(t)}>
              {t[0].toUpperCase() + t.slice(1)}
            </button>
          ))}
        </div>

        <div className="details-body scrollable">
          {tab === "overview" && (
            <dl className="details-grid">
              <dt>Status</dt>
              <dd>
                <span className={`status-dot ${status.Status}`} />
                {statusLabel(status.Status)}
              </dd>
              <dt>PID</dt>
              <dd className="mono">{status.PID || "—"}</dd>
              <dt>Host</dt>
              <dd className="mono">{p.hostName || "—"}</dd>
              <dt>User</dt>
              <dd className="mono">{p.user || "—"}</dd>
              <dt>Port</dt>
              <dd className="mono">{p.port || 22}</dd>
              <dt>ProxyJump</dt>
              <dd className="mono">{p.proxyJump || "—"}</dd>
              <dt>Identity</dt>
              <dd className="mono">{p.identityFile || "None"}</dd>
              <dt>Certificate</dt>
              <dd className="mono">{p.certificate || "None"}</dd>
              <dt>ForwardAgent</dt>
              <dd>{p.forwardAgent ? "yes" : "no"}</dd>
              <dt>Forwards</dt>
              <dd>{p.forwards?.length ?? 0}</dd>
              {status.Err ? (
                <>
                  <dt>Error</dt>
                  <dd className="err-text">{status.Err}</dd>
                </>
              ) : null}
            </dl>
          )}

          {tab === "logs" && (
            <pre className="details-pre">{status.Logs?.trim() || "No output captured yet."}</pre>
          )}

          {tab === "command" && (
            <pre className="details-pre">{["/usr/bin/ssh", ...args].join(" \\\n  ")}</pre>
          )}
        </div>

        <div className="modal-actions">
          <button className="btn btn-primary" onClick={onClose}>
            Close
          </button>
        </div>
      </div>
    </div>
  );
}
