# Governance chain overhaul — dynamic, animated, switchable

Date: 2026-08-03
Status: approved, not yet implemented

## Problem

The governance chain is the console's signature element — it renders the one
thing true of every AgentOS request regardless of which page you are on. Today
it is a 1px rule with six 7px squares pinned in a ~40px topbar. It is
evidentially rigorous and visually inert: on a quiet system it reads as dead
rather than as ready, and it is not something anyone would look at twice.

The goal is a chain that is dynamic, animated and visually impressive **without
surrendering the evidentiary discipline that makes it worth showing at all.**

## Non-negotiable constraint

`console/src/lib/chain.ts` exists to stop the console drawing checks it cannot
prove ran. Its own words (chain.ts:56):

> a chain that under-reports, or over-claims what a lit stage proves, is worse
> than no chain, because it implies checks ran that did not.

That module is **not modified by this work**. Every visual added here is driven
by recorded audit evidence. Specifically:

- No synthetic or decorative "requests." A mark that looks like a request IS a
  request, one-to-one with an audit row.
- Ambient idle motion is permitted only where it cannot be mistaken for a
  request traversal.
- The `unproven` distinction survives into every renderer (see §4).

## Decisions taken

| Question | Decision |
|---|---|
| Purpose | Honest under live traffic, but a *designed* resting state — not a dark inert rule |
| Footprint | Thin-ish always-on strip **plus** an expandable detail panel |
| Sizing | One height-responsive renderer per style: topbar ~76px, expanded ~220px |
| Transport | Poll-driven replay on the existing 5s shared poll — no backend change |
| Styles | All three implemented, switchable in Settings |
| Default | `phosphor` |
| Fourth option | `minimal` — today's hairline chain, retained |

### Why poll-driven replay

There is no audit stream. `GET /admin/audit` is the only audit route in the
gateway; its SSE (`server.go:767`) is chat-completion proxying only, and
`console/src/lib/sse.ts` is used solely by Playground. Adding
`GET /admin/audit/stream` is real Go work — handler, store fan-out, auth,
tests — and is separate scope from a visual overhaul.

`AuditEntry` already carries `ts` and `latency_ms`. That is enough to replay
each new row at its true relative time with a traversal duration scaled to its
real latency, which at a glance is indistinguishable from live. The registry
already models `transport: "poll" | "sse"`, so a future stream can be swapped in
underneath without touching the renderers.

Accepted cost: up to ~5s behind reality, and a burst arrives as a burst.

## 1. Architecture

```
lib/chain.ts            UNCHANGED — audit row -> ChainState
lib/chainReplay.ts      NEW, pure — two feed snapshots -> ChainPacket[]
components/chain/
  useChainPackets.ts    poll subscription + replay scheduling
  ChainCanvas.tsx       canvas host: rAF, resize, visibility, reduced-motion
  renderers/types.ts    the ChainRenderer seam
  renderers/corridor.ts
  renderers/phosphor.ts
  renderers/flow.ts
  Chain.tsx             orchestration + the accessible DOM
```

`Chain.tsx` today is 141 lines doing subscription, state and render together.
Splitting the poll/replay concern into a hook and the drawing into renderers
keeps each file focused; the renderers are the only place that grows.

### The renderer seam

```ts
export interface ChainRenderer {
  /** Draw one frame. dt is ms since previous frame. */
  draw(ctx: CanvasRenderingContext2D, w: number, h: number,
       packets: readonly ChainPacket[], state: ChainState, dt: number): void;
  /** Drop accumulated visual state (style switch, resize, remount). */
  reset(): void;
}
```

Adding a fourth style later is one new file and one entry in the style registry.

### ChainPacket

**Row identity.** `AuditEntry` has no unique id — its fields are `ts`,
`key_name`, `model`, `input_tokens`, `output_tokens`, `cost_usd`, `latency_ms`,
`status`, `kind`. Two identical calls in the same millisecond are therefore
indistinguishable by value, so per-row keys alone cannot drive the diff.

The diff is **positional**, not key-based: the feed is newest-first and bounded
at 100, so locate the previous snapshot's head row inside the new feed and take
everything above it as new. A composite of all nine fields is the matcher used
to find that anchor. If the anchor is not found — the feed advanced by more than
100 rows between polls, or the key changed — treat it as a re-seed: adopt the
new snapshot as baseline and replay nothing, because replaying 100 rows of
history as live traffic would be a lie. Log nothing; this is normal under load.

`ChainPacket.id` is a synthetic per-release counter for React keying only. It
carries no meaning and must not be used for dedupe.

```ts
export interface ChainPacket {
  id: string;              // synthetic; React keying only, never dedupe
  ts: number;              // epoch ms, from AuditEntry.ts
  stopIndex: number;       // index into CHAIN_STAGES, or -1 when fully cleared
  outcome: ChainOutcome;   // reuses lib/chain.ts
  latencyMs: number;
  unproven: readonly ChainStage[];  // carried through from chainStateFromEntry
}
```

Derived from `chainStateFromEntry`, so a packet can never claim more than the
audit row it came from.

## 2. Data flow

1. `useChainPackets` subscribes to the same `admin/audit?limit=100#<key>` resource
   at the same 5000ms cadence Overview uses — the registry dedupes them onto one
   poll. **The cadence must not change**; the registry keys by it, and diverging
   would double admin API load.
2. On each snapshot, `chainReplay.diffFeed(prev, next)` returns genuinely-new
   rows, newest-first input, oldest-first output.
3. Admin-plane rows (`secret_reload`) are excluded — reusing the existing
   `ADMIN_PLANE_KINDS` filter via `chainFeedCount`/`latestChainState` semantics.
   An operator clicking Reload on the Secrets page must not launch a packet.
4. Each new row is scheduled for release at its own `ts` offset within the
   window, so a burst replays with its real internal spacing.
5. The `upstream` leg's traversal duration scales with that row's `latency_ms`.
6. `state` (the existing `ChainState` for the newest row) continues to drive the
   resting/summary appearance exactly as today.

### Failure and edge behaviour

- **Failed or in-flight poll** — hold the last known reading. Never render a
  denial from absent data. (Existing `Chain.tsx` behaviour; preserved.)
- **No admin key** — idle chain, no packets, resting state only.
- **Empty feed** — idle. Quiet must not look like healthy.
- **Clock skew / future `ts`** — clamp release offset to `[0, cadence]`.
- **First load** — seed the baseline snapshot without replaying its 100 rows;
  history is not live traffic.
- **Style switch mid-flight** — `reset()`, drop in-flight visual state, keep
  the pending packet queue.

## 3. Sizing

One renderer per style, height-responsive rather than two separate designs:

- **Topbar** ~76px, always visible on every page. Grown from ~40px.
- **Expanded panel** ~220px plus readouts (`IN FLIGHT`, p95 latency, denial
  count), opened from a disclosure control in the strip.

The topbar keeps the `governance chain` eyebrow and the `synced` connection
glyph (`App.tsx:275-284`); those are unchanged.

## 4. The `unproven` vocabulary (critical)

`stageRenders` returns `cleared | stopped | unlit`, where `unlit` covers stages
the evidence does not prove ran. This is not an edge case — `guardrail` is
unproven on *every* plain `chat` row, and `upstream` is unproven on
`guardrail_flag`/`guardrail_error`.

Each renderer must therefore distinguish "the packet went past here" from "this
gate demonstrably fired":

| Style | cleared | unproven | stopped |
|---|---|---|---|
| Corridor | gate flares on contact | packet passes, gate does **not** flare; frame stays a dim outline | gate slams, packet shatters into falling sparks |
| Phosphor | pulse deposits phosphor at the tick | pulse crosses tick depositing **no** persistence | hard clip + burn mark decaying over ~2.6s |
| Flow | particle crosses a solid gate column | particle crosses a **dashed** gate column | particle turns deny-red, deflects out, falls away |

A renderer that lights every stage a request passed is a defect, not a polish
item.

## 5. Settings

- Key: `agentos-chain-style` in `localStorage`, following the `agentos-theme`
  precedent in `SettingsModal.tsx:45`.
- Values: `phosphor` (default) | `corridor` | `flow` | `minimal`.
- `minimal` renders today's hairline chain, retained for low-power machines and
  anyone who does not want an animated canvas above their content.
- `minimal` is a **user choice, not the reduced-motion mechanism**. An OS
  `prefers-reduced-motion` preference does not switch the stored style; it makes
  whichever style is selected render one static frame (§6). The two are
  independent, and a user who has chosen `phosphor` still sees phosphor —
  still, not minimal.
- Unknown/corrupt stored value falls back to the default rather than throwing.

## 6. Accessibility

- The canvas is `aria-hidden`. It is decoration over an accessible substrate.
- The existing semantic `<ol className="chain-stages">` with per-stage
  `title` text and the `aria-live="polite"` outcome glyph **remain exactly as
  they are**. Screen-reader behaviour must not regress; a denial is still
  announced.
- `prefers-reduced-motion: reduce` → no rAF loop at all. One static frame
  reflecting the current `ChainState`. This mirrors the existing
  `useReducedMotion()` handling.
- Stage labels remain readable at `--text-dim` (the `--text-faint` contrast
  failure noted in `Chain.css:86-90` must not be reintroduced by the redesign).

## 7. Performance

- Single canvas at topbar size; the expanded panel reuses the same element
  resized, not a second loop.
- rAF pauses when the tab is hidden, matching the poll's existing pause.
- Particle/streak/pulse counts capped per renderer. Under a burst, degrade by
  dropping marks — never by dropping frames.
- DPR-aware sizing via `ResizeObserver`; no layout thrash per frame.

## 8. Testing

- `chainReplay.ts` is pure and gets real vitest coverage: diffing, `ts`
  ordering, no duplicate release across polls, admin-plane exclusion, clock-skew
  clamping, first-load seeding. Matches the existing `chain.test.ts` convention.
- Each renderer gets a smoke test: `draw()` runs headless against a stub 2D
  context without throwing, for idle / cleared / stopped / unproven inputs.
- Settings persistence: round-trip, and unknown value falls back to default.
- The existing 249 vitest tests stay green.

## Out of scope

- `GET /admin/audit/stream` and any gateway change. Deliberately deferred; the
  registry's `transport` field leaves the door open.
- Any modification to `lib/chain.ts` evidence semantics.
- Changes to the Docs page governance visuals
  (`pages/docs/visuals/GovernanceChainVisual.tsx`), which are separate
  illustrations with their own purpose.
