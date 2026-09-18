import { useEffect, useRef, useState } from "react";
import type { PasswordPrompt } from "../types";

type Props = {
  prompt: PasswordPrompt;
  onSubmit: (secret: string, save: boolean) => void;
  onCancel: () => void;
};

export function PasswordModal({ prompt, onSubmit, onCancel }: Props) {
  const [secret, setSecret] = useState("");
  const [save, setSave] = useState(true);
  const ref = useRef<HTMLInputElement>(null);

  useEffect(() => {
    setSecret("");
    setSave(true);
    ref.current?.focus();
  }, [prompt.requestId]);

  const label = prompt.kind === "passphrase" ? "passphrase" : "password";

  return (
    <div className="modal-backdrop">
      <div className="modal" role="dialog" aria-modal="true">
        <h3>Authentication</h3>
        <p>{prompt.prompt || (prompt.kind === "passphrase" ? "Enter passphrase" : "Enter password")}</p>
        <input
          ref={ref}
          type="password"
          value={secret}
          onChange={(e) => setSecret(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") onSubmit(secret, save);
            if (e.key === "Escape") onCancel();
          }}
        />
        <label className="modal-check">
          <input type="checkbox" checked={save} onChange={(e) => setSave(e.target.checked)} />
          <span>Save {label} in Keychain</span>
        </label>
        <div className="modal-actions">
          <button className="btn" onClick={onCancel}>
            Cancel
          </button>
          <button className="btn btn-primary" onClick={() => onSubmit(secret, save)}>
            OK
          </button>
        </div>
      </div>
    </div>
  );
}
