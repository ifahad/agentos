// TEMP: icon review, remove in S2 Task 6
//
// Visual review sheet for the icon set — every glyph at the three sizes the
// console actually uses (13 inline, 15 nav, 20 oversized for scrutiny). Not a
// route anyone links to; reachable only by typing /_icons. Delete this file
// and its App.tsx wiring once the action-glyph family is approved.
import type { JSX } from "react";
import { BrandMark, Icon } from "../ui/icons";
import type { IconName } from "../ui/icons";

const NAMES: IconName[] = [
  "overview", "keys", "audit", "playground", "documents", "improve", "orgs", "users",
  "secrets", "provisioning", "settings", "multiverse", "operators",
  "copy", "refresh", "run", "pause", "approve", "deny", "export", "search", "filter",
  "sort", "close", "external-link", "chevron", "trash", "plus",
];

// TEMP: brand review, remove in S2 Task 6
//
// Two alternates to the shipped `BrandMark` (ui/icons.tsx), kept local to this
// sheet since they are not candidates for the permanent export — only the
// recommendation ships there. Same grid as `BrandMark`: 16x16, artwork in
// 2..14, 1.25 stroke, square caps, mitre joins, half-unit snapping, no fill.

// "chain-rail": the same rule, but round nodes threaded on it like beads
// rather than square nodes seated on top of it — a softer reading of the
// same motif.
function BrandMarkRail({ size = 20 }: { size?: number }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 16 16"
      fill="none"
      stroke="currentColor"
      strokeWidth={1.25}
      strokeLinecap="square"
      strokeLinejoin="miter"
      aria-hidden
      focusable="false"
    >
      <path d="M2.5 8h11" />
      <circle cx="4" cy="8" r="1.5" />
      <circle cx="8" cy="8" r="1.5" />
      <circle cx="12" cy="8" r="1.5" />
    </svg>
  );
}

// "chain-emblem": the chain motif compressed and framed into a self-contained
// seal (ticks crossing a short rule inside a square plate) — reads as a mark
// on its own, useful anywhere the wordmark isn't alongside it (favicon, a
// future avatar slot).
function BrandMarkEmblem({ size = 20 }: { size?: number }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 16 16"
      fill="none"
      stroke="currentColor"
      strokeWidth={1.25}
      strokeLinecap="square"
      strokeLinejoin="miter"
      aria-hidden
      focusable="false"
    >
      <rect x="2.5" y="2.5" width="11" height="11" />
      <path d="M4.5 8h7" />
      <path d="M6 6.5v3" />
      <path d="M8 6.5v3" />
      <path d="M10 6.5v3" />
    </svg>
  );
}

const BRAND_VARIANTS: Array<{ label: string; render: (size: number) => JSX.Element }> = [
  { label: "chain-3nodes (shipped default)", render: (size) => <BrandMark size={size} /> },
  { label: "chain-rail", render: (size) => <BrandMarkRail size={size} /> },
  { label: "chain-emblem", render: (size) => <BrandMarkEmblem size={size} /> },
];

// TEMP: brand review, remove in S2 Task 6
function BrandVariants() {
  return (
    <div style={{ padding: "24px 24px 0", borderBottom: "1px solid var(--border)", paddingBottom: 24 }}>
      <div className="mono" style={{ fontSize: 10, color: "var(--text-faint)", marginBottom: 12 }}>
        brand variants — pick one for the sidebar (see Sidebar.tsx)
      </div>
      <div style={{ display: "flex", gap: 32, alignItems: "center" }}>
        {BRAND_VARIANTS.map((v) => (
          <div key={v.label} style={{ display: "flex", flexDirection: "column", alignItems: "center", gap: 8 }}>
            <div
              style={{
                display: "flex",
                alignItems: "center",
                gap: 8,
                padding: "10px 14px",
                background: "var(--inset)",
                border: "1px solid var(--border)",
                borderRadius: "var(--radius-sm)",
              }}
            >
              {v.render(18)}
              <span
                style={{
                  fontSize: 14,
                  fontWeight: 600,
                  letterSpacing: "0.14em",
                  textTransform: "uppercase",
                }}
              >
                AgentOS
              </span>
            </div>
            <div style={{ display: "flex", gap: 10, alignItems: "center" }}>
              {v.render(18)}
              {v.render(24)}
            </div>
            <span className="mono" style={{ fontSize: 10, color: "var(--text-faint)" }}>{v.label}</span>
          </div>
        ))}
      </div>
    </div>
  );
}

export function IconSheet() {
  return (
    <div>
      <BrandVariants />
      <div style={{ padding: 24, display: "grid", gridTemplateColumns: "repeat(6, 1fr)", gap: 20 }}>
        {NAMES.map((n) => (
          <div key={n} style={{ display: "flex", flexDirection: "column", alignItems: "center", gap: 6 }}>
            <div style={{ display: "flex", gap: 10, alignItems: "center" }}>
              <Icon name={n} size={13} />
              <Icon name={n} size={15} />
              <Icon name={n} size={20} />
            </div>
            <span className="mono" style={{ fontSize: 10, color: "var(--text-faint)" }}>{n}</span>
          </div>
        ))}
      </div>
    </div>
  );
}
