import { useEffect, useRef, useState } from "react";
import { useLiveResource } from "../../hooks/useLiveResource";
import { apiFetch, gatewayAdminRequest } from "../../lib/api";
import { IDLE_CHAIN, latestChainState } from "../../lib/chain";
import type { ChainState } from "../../lib/chain";
import type { ChainPacket } from "../../lib/chainReplay";
import { diffFeed, releaseSchedule, toPacket } from "../../lib/chainReplay";
import type { AuditEntry } from "../../lib/types";

/**
 * MUST stay 5000 and MUST match Chain.tsx's (and Overview's) audit
 * subscription. The live registry keys by (key, cadence) and dedupes
 * identical subscriptions onto one poll; diverging here would silently
 * double admin API load against the gateway.
 */
const CADENCE_MS = 5000;

/**
 * How long a released packet stays in the returned `packets` array before
 * it is dropped.
 *
 * ChainCanvas retains an adopted packet in its own local `live` array for at
 * most MAX_TRAVEL_MS (4000) + RETIRE_AFTER_S*1000 (1200) ~= 5.2s
 * (src/components/chain/lifecycle.ts), independent of how long the packet
 * stays in this array — once adopted, its on-screen life is driven entirely
 * by the host's own progress/deadFor bookkeeping. What this array's lifetime
 * has to cover is (a) the window in which the host's rAF loop can still
 * *adopt* the packet at all (its `seen` set is keyed off ids present in
 * `live` OR this array, and its rAF loop pauses whenever the tab is hidden),
 * and (b) reduced-motion mode, where the host redraws a still frame straight
 * from this array on every change with no independent `live` timeline of its
 * own, so this array's lifetime IS the on-screen lifetime there.
 *
 * 8000ms leaves ~2.8s of slack beyond the host's ~5.2s worst case — enough
 * to absorb scheduling jitter or a briefly backgrounded tab without either
 * disappearing before the host ever sees it, or (under reduced motion)
 * vanishing implausibly fast. Coherent as specified; not adjusted.
 */
const PACKET_TTL_MS = 8000;

/**
 * Real requests, released on their real timing.
 *
 * Every packet is one audit row. Nothing here invents traffic: with no key, a
 * failed poll, or an empty feed, no packet is ever produced.
 */
export function useChainPackets(adminKey: string): {
  packets: ChainPacket[];
  state: ChainState;
  active: boolean;
} {
  const audit = useLiveResource<AuditEntry[]>(
    `admin/audit?limit=100#${adminKey}`,
    () => apiFetch<AuditEntry[]>(gatewayAdminRequest("/admin/audit?limit=100", adminKey)),
    { enabled: Boolean(adminKey), cadence: CADENCE_MS },
  );

  const [packets, setPackets] = useState<ChainPacket[]>([]);
  const [state, setState] = useState<ChainState>(IDLE_CHAIN);
  const [active, setActive] = useState(false);

  const prevFeed = useRef<AuditEntry[] | null>(null);
  // Outstanding setTimeout ids. Each timer removes its own id the instant it
  // fires, so this never grows past the count of timers genuinely in flight
  // — unlike an ever-appended array, it does not accumulate for the life of
  // the component. Cleared wholesale on an adminKey change and on unmount.
  const timers = useRef<Set<number>>(new Set());
  const counter = useRef(0);
  // Id of the most recently scheduled "go quiet" timer, so a new burst can
  // cancel the previous deadline instead of stacking a second one that would
  // flip `active` off mid-burst while newer traffic is still genuinely live.
  const quietTimer = useRef<number | null>(null);

  // Drop everything when the credential changes — packets from the old key
  // describe traffic the new key may not even be allowed to see.
  useEffect(() => {
    prevFeed.current = null;
    counter.current = 0;
    for (const t of timers.current) window.clearTimeout(t);
    timers.current.clear();
    quietTimer.current = null;
    setPackets([]);
    setState(IDLE_CHAIN);
    setActive(false);
  }, [adminKey]);

  useEffect(() => {
    const entries = audit.data;
    // A failed or in-flight poll says nothing about governance: hold the last
    // reading rather than reporting a denial nobody caused.
    if (!adminKey || !entries) return;

    const fresh = diffFeed(prevFeed.current, entries);
    prevFeed.current = entries.slice();
    setState(latestChainState(entries));

    if (fresh.length === 0) return;

    const newPackets = fresh.map((entry) => toPacket(entry, `pkt-${counter.current++}`));
    const delays = releaseSchedule(newPackets, CADENCE_MS);

    setActive(true);
    newPackets.forEach((packet, i) => {
      const release = window.setTimeout(() => {
        timers.current.delete(release);
        setPackets((cur) => [...cur, packet]);
        const expire = window.setTimeout(() => {
          timers.current.delete(expire);
          setPackets((cur) => cur.filter((p) => p.id !== packet.id));
        }, PACKET_TTL_MS);
        timers.current.add(expire);
      }, delays[i]);
      timers.current.add(release);
    });

    // This burst pushes out the "go quiet" deadline rather than adding a
    // second, independent one: an earlier deadline landing mid-burst would
    // otherwise report `active: false` while this burst's packets are still
    // in flight or freshly on screen.
    if (quietTimer.current !== null) {
      window.clearTimeout(quietTimer.current);
      timers.current.delete(quietTimer.current);
    }
    const quiet = window.setTimeout(() => {
      timers.current.delete(quiet);
      quietTimer.current = null;
      setActive(false);
    }, CADENCE_MS + PACKET_TTL_MS);
    quietTimer.current = quiet;
    timers.current.add(quiet);
  }, [adminKey, audit.data]);

  // Clear every pending timer on unmount so a navigated-away chain sets no
  // state. Reads timers.current live inside the cleanup closure rather than
  // capturing the Set's contents at mount, so it clears whatever is actually
  // outstanding at unmount time — including timers scheduled after an
  // adminKey change, whose reset effect (above) clears the same Set in place
  // rather than replacing it with a new object.
  useEffect(() => {
    return () => {
      for (const t of timers.current) window.clearTimeout(t);
      timers.current.clear();
    };
  }, []);

  return { packets, state, active };
}
