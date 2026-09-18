import { useEffect, useMemo, useRef, useState } from "react";

type Props = {
  tags: string[];
  allTags: string[];
  onChange: (tags: string[]) => void;
  /** Increment to force-open the tag popover (e.g. File → New Tag). */
  openSignal?: number;
};

/** Tag chips + a popover (the “hidden” tag menu) for adding/removing. */
export function TagEditor({ tags, allTags, onChange, openSignal = 0 }: Props) {
  const [open, setOpen] = useState(false);
  const [draft, setDraft] = useState("");
  const rootRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (openSignal > 0) setOpen(true);
  }, [openSignal]);

  const suggestions = useMemo(() => {
    const q = draft.trim().toLowerCase();
    return allTags
      .filter((t) => !tags.includes(t))
      .filter((t) => !q || t.toLowerCase().includes(q))
      .slice(0, 8);
  }, [allTags, tags, draft]);

  useEffect(() => {
    if (!open) return;
    inputRef.current?.focus();
    const onDown = (e: MouseEvent) => {
      if (!rootRef.current?.contains(e.target as Node)) setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    window.addEventListener("mousedown", onDown);
    window.addEventListener("keydown", onKey);
    return () => {
      window.removeEventListener("mousedown", onDown);
      window.removeEventListener("keydown", onKey);
    };
  }, [open]);

  const add = (raw: string) => {
    const tag = raw.trim();
    if (!tag) return;
    if (tags.some((t) => t.toLowerCase() === tag.toLowerCase())) {
      setDraft("");
      return;
    }
    onChange([...tags, tag]);
    setDraft("");
  };

  const remove = (tag: string) => onChange(tags.filter((t) => t !== tag));

  return (
    <div className="tags-row" ref={rootRef}>
      <button
        className="tags-label-btn"
        onClick={() => setOpen((v) => !v)}
        title="Manage tags"
      >
        Tags
        <Chevron open={open} />
      </button>

      <div className="tag-chips">
        {tags.map((tag) => (
          <span key={tag} className="tag-chip">
            {tag}
            <button className="tag-chip-x" onClick={() => remove(tag)} aria-label={`Remove ${tag}`}>
              ×
            </button>
          </span>
        ))}
        <button
          className="icon-btn tag-add-btn"
          title="Add tag"
          aria-label="Add tag"
          onClick={() => setOpen(true)}
        >
          <SmallPlus />
        </button>
      </div>

      {open && (
        <div className="tag-menu" role="dialog" aria-label="Tag menu">
          <input
            ref={inputRef}
            className="tag-menu-input"
            placeholder="New tag…"
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                e.preventDefault();
                add(draft);
              }
            }}
          />
          <div className="tag-menu-list scrollable">
            {suggestions.length === 0 && !draft.trim() ? (
              <div className="tag-menu-empty">No other tags yet</div>
            ) : null}
            {draft.trim() && !allTags.some((t) => t.toLowerCase() === draft.trim().toLowerCase()) ? (
              <button className="tag-menu-item" onClick={() => add(draft)}>
                Create “{draft.trim()}”
              </button>
            ) : null}
            {suggestions.map((t) => (
              <button key={t} className="tag-menu-item" onClick={() => add(t)}>
                {t}
              </button>
            ))}
            {tags.length > 0 && (
              <>
                <div className="tag-menu-heading">On this tunnel</div>
                {tags.map((t) => (
                  <button key={`rm-${t}`} className="tag-menu-item is-active" onClick={() => remove(t)}>
                    <span>{t}</span>
                    <span className="tag-menu-check">✓</span>
                  </button>
                ))}
              </>
            )}
          </div>
        </div>
      )}
    </div>
  );
}

function SmallPlus() {
  return (
    <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.4">
      <path d="M12 5v14M5 12h14" />
    </svg>
  );
}

function Chevron({ open }: { open: boolean }) {
  return (
    <svg
      width="10"
      height="10"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2.5"
      style={{ transform: open ? "rotate(180deg)" : undefined, transition: "transform 0.12s ease" }}
    >
      <path d="M6 9l6 6 6-6" />
    </svg>
  );
}
