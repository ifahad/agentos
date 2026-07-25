/**
 * AgentOS icon set — drawn for this console, not imported from a library.
 *
 * A stock icon set is the fastest way to make a product look like every other
 * product, so these are schematic marks rather than friendly pictograms: they
 * borrow from panel legends and wiring diagrams, which is the world AgentOS
 * actually operates in.
 *
 * Grid rules, applied to every glyph without exception:
 *   - 16x16 viewBox, artwork confined to 2..14 so glyphs optically align
 *   - 1.25 stroke, square caps, mitre joins — machined, never rounded
 *   - geometry snaps to whole or half units; no arbitrary curves
 *   - stroke: currentColor, so a glyph inherits whatever state colors its row
 *
 * Sizes are set by the caller in px. Glyphs are decorative by default and are
 * hidden from assistive tech; pass a `title` only when the icon is the sole
 * carrier of meaning (a status glyph with no adjacent text).
 */

import type { JSX } from "react";

export type IconName =
  | "overview"
  | "keys"
  | "audit"
  | "playground"
  | "documents"
  | "improve"
  | "orgs"
  | "users"
  | "secrets"
  | "provisioning"
  | "settings"
  | "multiverse"
  | "operators";

/** Glyph geometry. Each entry is the inner artwork of a 16x16 icon. */
const GLYPHS: Record<IconName, JSX.Element> = {
  // Gauge — the panel you read first. Arc with a needle off-centre, because a
  // needle at rest in the middle would read as a decorative dial.
  overview: (
    <>
      <path d="M2.5 12.5a5.5 5.5 0 0 1 11 0" />
      <path d="M8 12.5 11.5 7" />
    </>
  ),

  // Key — bit and collar. The teeth point down so the silhouette differs from
  // the padlock used for secrets.
  keys: (
    <>
      <circle cx="5" cy="6" r="2.5" />
      <path d="M6.8 7.8 12.5 13.5" />
      <path d="M10.5 11.5 9 13" />
    </>
  ),

  // Ledger — a spine with entries hanging off it. This is the audit trail, and
  // the same spine motif recurs in the activity feed.
  audit: (
    <>
      <path d="M3.5 2.5v11" />
      <path d="M3.5 5h9" />
      <path d="M3.5 8.5h6.5" />
      <path d="M3.5 12h8" />
    </>
  ),

  // Prompt — chevron and entry rule. A play triangle would suggest media; this
  // is a place where you type at the system.
  playground: (
    <>
      <path d="M3 4.5 6.5 8 3 11.5" />
      <path d="M8.5 12h4.5" />
    </>
  ),

  // Sheets — two offset planes. Corpus, not a single file, so no page fold.
  documents: (
    <>
      <path d="M5.5 2.5h5l3 3v6h-8z" />
      <path d="M10.5 2.5v3h3" />
      <path d="M2.5 5.5v8h8" />
    </>
  ),

  // Return loop — an arc that closes on itself with a tick. The self-improvement
  // cycle: propose, evaluate, fold back in.
  improve: (
    <>
      <path d="M13 8a5 5 0 1 1-1.8-3.85" />
      <path d="M13.2 2v2.6h-2.6" />
    </>
  ),

  // Hierarchy — one parent, two children. Orgs own keys and users.
  orgs: (
    <>
      <rect x="6" y="2.5" width="4" height="3" />
      <rect x="2.5" y="10.5" width="4" height="3" />
      <rect x="9.5" y="10.5" width="4" height="3" />
      <path d="M8 5.5v2.5" />
      <path d="M4.5 10.5V8h7v2.5" />
    </>
  ),

  // Two figures, one behind. Deliberately asymmetric so it never reads as a
  // single silhouette at 14px.
  users: (
    <>
      <circle cx="6" cy="5.5" r="2.5" />
      <path d="M2.5 13.5a3.5 3.5 0 0 1 7 0" />
      <path d="M10.5 4.2a2.4 2.4 0 0 1 0 4.6" />
      <path d="M11.2 10.4a3.5 3.5 0 0 1 2.3 3.1" />
    </>
  ),

  // Padlock, shackle closed. Secrets are sealed at rest; the closed shackle is
  // the whole point.
  secrets: (
    <>
      <rect x="3" y="7" width="10" height="6.5" />
      <path d="M5.5 7V5a2.5 2.5 0 0 1 5 0v2" />
      <path d="M8 9.5v1.5" />
    </>
  ),

  // Distribution — a source fanning out to endpoints. SCIM pushes identity
  // outward, so the arrows leave the node rather than arrive at it.
  provisioning: (
    <>
      <circle cx="3.5" cy="8" r="1.5" />
      <circle cx="12.5" cy="4" r="1.5" />
      <circle cx="12.5" cy="12" r="1.5" />
      <path d="M5 7.3 11 4.6" />
      <path d="M5 8.7 11 11.4" />
    </>
  ),

  // Faders. Settings are values you set, not gears you turn.
  settings: (
    <>
      <path d="M2.5 5.5h11" />
      <path d="M2.5 10.5h11" />
      <circle cx="6" cy="5.5" r="1.75" />
      <circle cx="10" cy="10.5" r="1.75" />
    </>
  ),

  // Council — several bounded agents around one shared verdict. Drawn as peers,
  // not a hierarchy, because no member outranks another.
  multiverse: (
    <>
      <circle cx="8" cy="8" r="2" />
      <circle cx="8" cy="2.8" r="1.3" />
      <circle cx="13.2" cy="8" r="1.3" />
      <circle cx="8" cy="13.2" r="1.3" />
      <circle cx="2.8" cy="8" r="1.3" />
    </>
  ),

  // Trigger — a standing order firing. Ticks strike a square node, the same
  // node the governance chain seats on its rule: the signal arrives, the
  // machine runs. Provisioning fans OUT of a circle; this strikes INTO a square.
  operators: (
    <>
      <rect x="6" y="9" width="4" height="4" />
      <path d="M8 2.5v3" />
      <path d="M3.5 4.5 5.5 6.5" />
      <path d="M12.5 4.5 10.5 6.5" />
    </>
  ),
};

interface IconProps {
  name: IconName;
  /** Edge length in px. Nav uses 15; inline labels use 13. */
  size?: number;
  /** Accessible name. Omit when adjacent text already names the thing. */
  title?: string;
  className?: string;
}

/** Render one glyph from the set. */
export function Icon({ name, size = 15, title, className }: IconProps) {
  return (
    <svg
      className={className}
      width={size}
      height={size}
      viewBox="0 0 16 16"
      fill="none"
      stroke="currentColor"
      strokeWidth={1.25}
      strokeLinecap="square"
      strokeLinejoin="miter"
      role={title ? "img" : undefined}
      aria-hidden={title ? undefined : true}
      aria-label={title}
      focusable="false"
    >
      {title ? <title>{title}</title> : null}
      {GLYPHS[name]}
    </svg>
  );
}

/** Machine states a run can be in. Drives every status glyph in the console. */
export type StateName = "live" | "ok" | "hold" | "deny";

const STATE_GLYPHS: Record<StateName, JSX.Element> = {
  // Filled — something is happening right now. The ring is animated by CSS.
  live: <circle cx="8" cy="8" r="3.5" fill="currentColor" stroke="none" />,
  // Closed ring with a tick: finished, and allowed.
  ok: (
    <>
      <circle cx="8" cy="8" r="5.5" />
      <path d="M5.5 8.2 7.2 10 10.5 6.2" />
    </>
  ),
  // Half-filled: the system did its part and stopped, waiting on a person.
  hold: (
    <>
      <circle cx="8" cy="8" r="5.5" />
      <path d="M8 2.5a5.5 5.5 0 0 1 0 11z" fill="currentColor" stroke="none" />
    </>
  ),
  // Barred: refused. A cross would read as "error"; this reads as "not permitted".
  deny: (
    <>
      <circle cx="8" cy="8" r="5.5" />
      <path d="M4.5 11.5 11.5 4.5" />
    </>
  ),
};

interface StateIconProps {
  state: StateName;
  size?: number;
  title?: string;
}

/**
 * Status glyph. These are the only marks in the console allowed to carry color,
 * and they take it from the matching --live/--ok/--hold/--deny token via the
 * `.state-<name>` class, so a state can never be drawn in the wrong hue.
 */
export function StateIcon({ state, size = 13, title }: StateIconProps) {
  return (
    <svg
      className={`state-icon state-${state}`}
      width={size}
      height={size}
      viewBox="0 0 16 16"
      fill="none"
      stroke="currentColor"
      strokeWidth={1.25}
      strokeLinecap="square"
      strokeLinejoin="miter"
      role={title ? "img" : undefined}
      aria-hidden={title ? undefined : true}
      aria-label={title}
      focusable="false"
    >
      {title ? <title>{title}</title> : null}
      {STATE_GLYPHS[state]}
    </svg>
  );
}
