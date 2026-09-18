/** Shared UI icons — Core Tunnel–inspired, no question-mark glyphs. */

type IconProps = {
  size?: number;
  className?: string;
};

export function PlusIcon({ size = 14, className }: IconProps) {
  return (
    <svg className={className} width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.4" strokeLinecap="round">
      <path d="M12 5v14M5 12h14" />
    </svg>
  );
}

/** Overlapping documents — Core Tunnel Duplicate. */
export function DuplicateIcon({ size = 15, className }: IconProps) {
  return (
    <svg className={className} width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
      <rect x="8" y="8" width="11" height="13" rx="1.5" />
      <path d="M6.5 16H5.5A1.5 1.5 0 0 1 4 14.5v-11A1.5 1.5 0 0 1 5.5 2H15a1.5 1.5 0 0 1 1.5 1.5V5" />
    </svg>
  );
}

/** Horizontal sliders — Core Tunnel Edit. */
export function EditSlidersIcon({ size = 15, className }: IconProps) {
  return (
    <svg className={className} width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round">
      <path d="M4 7h16" />
      <path d="M4 12h16" />
      <path d="M4 17h16" />
      <circle cx="8" cy="7" r="2.2" fill="currentColor" stroke="none" />
      <circle cx="16" cy="12" r="2.2" fill="currentColor" stroke="none" />
      <circle cx="11" cy="17" r="2.2" fill="currentColor" stroke="none" />
    </svg>
  );
}

export function GearIcon({ size = 15, className }: IconProps) {
  return (
    <svg className={className} width={size} height={size} viewBox="0 0 24 24" fill="currentColor">
      <path d="M19.14 12.94c.04-.31.06-.63.06-.94s-.02-.63-.06-.94l2.03-1.58a.5.5 0 0 0 .12-.64l-1.92-3.32a.5.5 0 0 0-.6-.22l-2.39.96a7.07 7.07 0 0 0-1.63-.94l-.36-2.54a.5.5 0 0 0-.5-.42h-3.84a.5.5 0 0 0-.5.42l-.36 2.54c-.59.24-1.13.55-1.63.94l-2.39-.96a.5.5 0 0 0-.6.22L2.77 8.84a.5.5 0 0 0 .12.64l2.03 1.58c-.04.31-.06.63-.06.94s.02.63.06.94l-2.03 1.58a.5.5 0 0 0-.12.64l1.92 3.32c.14.24.43.34.68.22l2.39-.96c.5.39 1.04.7 1.63.94l.36 2.54c.05.24.26.42.5.42h3.84c.24 0 .45-.18.5-.42l.36-2.54c.59-.24 1.13-.55 1.63-.94l2.39.96c.25.12.54.02.68-.22l1.92-3.32a.5.5 0 0 0-.12-.64l-2.03-1.58ZM12 15.6A3.6 3.6 0 1 1 12 8.4a3.6 3.6 0 0 1 0 7.2Z" />
    </svg>
  );
}

export function ChevronDownIcon({ size = 14, className, open }: IconProps & { open?: boolean }) {
  return (
    <svg
      className={className}
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2.6"
      strokeLinecap="round"
      strokeLinejoin="round"
      style={{
        opacity: 0.65,
        transform: open ? "rotate(180deg)" : undefined,
        transition: "transform 0.12s ease",
      }}
    >
      <path d="M6 9l6 6 6-6" />
    </svg>
  );
}

export function ChevronRightIcon({ size = 14, className }: IconProps) {
  return (
    <svg className={className} width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.6" strokeLinecap="round" strokeLinejoin="round">
      <path d="M9 6l6 6-6 6" />
    </svg>
  );
}

export function SearchIcon({ size = 14, className }: IconProps) {
  return (
    <svg className={className} width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round">
      <circle cx="11" cy="11" r="6.5" />
      <path d="M20 20l-3.2-3.2" />
    </svg>
  );
}

/** Host / destination glyph used on the connection card. */
export function HostIcon({ size = 18, className }: IconProps) {
  return (
    <svg className={className} width={size} height={size} viewBox="0 0 24 24" fill="none">
      <rect x="3" y="4" width="18" height="16" rx="3" fill="#3a82f7" />
      <rect x="5.5" y="7" width="13" height="7" rx="1.2" fill="#dce9ff" />
      <circle cx="8.2" cy="16.4" r="1.1" fill="#dce9ff" />
      <path d="M11.5 16.4h5.5" stroke="#dce9ff" strokeWidth="1.6" strokeLinecap="round" />
    </svg>
  );
}

type ForwardKind = "dynamic" | "local" | "remote" | "reverse-dynamic";

const FORWARD_COLORS: Record<ForwardKind, { bg: string; fg: string }> = {
  dynamic: { bg: "#5b5ce6", fg: "#ffffff" },
  local: { bg: "#30b04a", fg: "#ffffff" },
  remote: { bg: "#f08a24", fg: "#1a1208" },
  "reverse-dynamic": { bg: "#32b5c4", fg: "#ffffff" },
};

/** Colorful Core Tunnel–style badges for port-forward sections. */
export function ForwardTypeIcon({ type, size = 18 }: { type: ForwardKind; size?: number }) {
  const { bg, fg } = FORWARD_COLORS[type];
  const r = 5;
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" aria-hidden>
      <rect width="24" height="24" rx={r} fill={bg} />
      {type === "dynamic" && (
        <g fill="none" stroke={fg} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
          <path d="M12 6.5a5.5 5.5 0 0 1 5.5 5.5" />
          <path d="M12 17.5a5.5 5.5 0 0 1-5.5-5.5" />
          <path d="M16.2 7.2l1.6 2.4-2.6.2" />
          <path d="M7.8 16.8l-1.6-2.4 2.6-.2" />
        </g>
      )}
      {type === "local" && (
        <g fill="none" stroke={fg} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
          <path d="M5.5 12h10.5" />
          <path d="M13.5 8.5L17.5 12l-4 3.5" />
          <rect x="17.2" y="8" width="1.8" height="8" rx="0.6" fill={fg} stroke="none" />
        </g>
      )}
      {type === "remote" && (
        <g fill="none" stroke={fg} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
          <path d="M18.5 12H8" />
          <path d="M10.5 8.5L6.5 12l4 3.5" />
          <rect x="5" y="8" width="1.8" height="8" rx="0.6" fill={fg} stroke="none" />
        </g>
      )}
      {type === "reverse-dynamic" && (
        <g fill="none" stroke={fg} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
          <path d="M12 17.5a5.5 5.5 0 0 0 5.5-5.5" />
          <path d="M12 6.5a5.5 5.5 0 0 0-5.5 5.5" />
          <path d="M16.2 16.8l1.6-2.4-2.6-.2" />
          <path d="M7.8 7.2L6.2 9.6l2.6.2" />
        </g>
      )}
    </svg>
  );
}

export function ArrowRightIcon({ size = 14, className }: IconProps) {
  return (
    <svg className={className} width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M5 12h14M13 6l6 6-6 6" />
    </svg>
  );
}
