// TEMP: icon review, remove in S2 Task 6
//
// Visual review sheet for the icon set — every glyph at the three sizes the
// console actually uses (13 inline, 15 nav, 20 oversized for scrutiny). Not a
// route anyone links to; reachable only by typing /_icons. Delete this file
// and its App.tsx wiring once the action-glyph family is approved.
import { Icon } from "../ui/icons";
import type { IconName } from "../ui/icons";

const NAMES: IconName[] = [
  "overview", "keys", "audit", "playground", "documents", "improve", "orgs", "users",
  "secrets", "provisioning", "settings", "multiverse", "operators",
  "copy", "refresh", "run", "pause", "approve", "deny", "export", "search", "filter",
  "sort", "close", "external-link", "chevron", "trash", "plus",
];

export function IconSheet() {
  return (
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
  );
}
