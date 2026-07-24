import { useEffect, useMemo, useState } from "react";
import { AnimatePresence, motion, useReducedMotion } from "framer-motion";
import type { PageProps } from "../App";
import { ErrorNotice, NeedsKey, PageHead, errorMessage, useLoad } from "../components/common";
import { apiFetch, gatewayAdminRequest } from "../lib/api";
import { formatInt, formatTimestamp, formatUSD } from "../lib/format";
import type { AuditEntry, KeyInfo, KeyUsage } from "../lib/types";
import {
  Badge,
  Card,
  EASE,
  EmptyState,
  Panel,
  PanelHead,
  Skeleton,
  Stat,
  Table,
  Tbody,
  Tr,
  transitionFast,
  staggerItem,
  staggerItemReduced,
} from "../ui";
import { Sparkline, normalizeSeries, seriesFromEvents } from "../charts";
import { budgetMeters, feedEntryId, isInflight, mergeFeedEntries } from "./overviewFeed";
import "./Overview.css";

/** Activity feed poll interval (~5s per spec). */
const POLL_MS = 5_000;
/** Audit page size: powers both the sparkline (hourly buckets) and the feed. */
const AUDIT_LIMIT = 100;
/** Feed rows rendered at once (the stored feed keeps more for the sparkline). */
const FEED_DISPLAY = 20;
/** Sparkline width in hourly buckets. */
const SPARK_HOURS = 24;

/**
 * Poll GET /admin/audit (an endpoint the console already exposes) every
 * POLL_MS; merges into a deduped, capped feed so new entries can slide in.
 * Cleans up the interval on unmount / key change. Errors surface once and
 * clear on the next successful poll.
 */
function useActivity(adminKey: string) {
  const [entries, setEntries] = useState<AuditEntry[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!adminKey) {
      setEntries(null);
      setError(null);
      return;
    }
    let cancelled = false;
    const tick = () => {
      apiFetch<AuditEntry[]>(gatewayAdminRequest(`/admin/audit?limit=${AUDIT_LIMIT}`, adminKey))
        .then((fresh) => {
          if (cancelled) return;
          setEntries((cur) => mergeFeedEntries(cur ?? [], fresh, AUDIT_LIMIT));
          setError(null);
        })
        .catch((err: unknown) => {
          if (!cancelled) setError(errorMessage(err));
        });
    };
    tick();
    const id = window.setInterval(tick, POLL_MS);
    return () => {
      cancelled = true;
      window.clearInterval(id);
    };
  }, [adminKey]);

  return { entries, error, loading: entries === null && error === null };
}

/** Status dot class for a feed entry — live pulse, error, flag, or quiet. */
function dotClass(e: AuditEntry, live: boolean): string {
  if (live) return "ov-dot live";
  if (e.kind === "guardrail_block" || e.status >= 400) return "ov-dot err";
  if (e.kind === "guardrail_flag") return "ov-dot warn";
  return "ov-dot";
}

function meterTone(fraction: number): string {
  if (fraction >= 1) return "over";
  if (fraction >= 0.8) return "warn";
  return "";
}

export function Overview({ adminKey, openSettings }: PageProps) {
  const reduced = useReducedMotion();

  // Existing data hook — unchanged (per-key usage totals).
  const { data, error, loading } = useLoad(
    () =>
      adminKey
        ? apiFetch<KeyUsage[]>(gatewayAdminRequest("/admin/usage", adminKey))
        : Promise.resolve<KeyUsage[]>([]),
    [adminKey],
  );

  // Key budgets for the meters. Errors (e.g. a role that can't list keys)
  // degrade to an empty meter panel rather than a page-level error.
  const { data: keysData, loading: keysLoading } = useLoad(
    () =>
      adminKey
        ? apiFetch<KeyInfo[]>(gatewayAdminRequest("/admin/keys", adminKey)).catch(
            () => [] as KeyInfo[],
          )
        : Promise.resolve<KeyInfo[]>([]),
    [adminKey],
  );

  const activity = useActivity(adminKey);

  const usage = data ?? [];
  const totals = usage.reduce(
    (acc, u) => ({
      requests: acc.requests + u.requests,
      tokens: acc.tokens + u.input_tokens + u.output_tokens,
      spend: acc.spend + u.spend_usd,
    }),
    { requests: 0, tokens: 0, spend: 0 },
  );

  // Live request sparkline: audit events bucketed hourly, last 24 buckets.
  const sparkValues = useMemo(() => {
    const entries = activity.entries;
    if (!entries || entries.length === 0) return [];
    const series = seriesFromEvents(
      entries.map((e) => ({ timestamp: e.ts })),
      { bucket: "hour" },
    );
    return normalizeSeries(
      series.map((p) => p.value),
      SPARK_HOURS,
    );
  }, [activity.entries]);

  const feed = (activity.entries ?? []).slice(0, FEED_DISPLAY);
  const now = Date.now();
  const anyLive = feed.some((e) => isInflight(e, now));

  const meters = budgetMeters(keysData ?? []);

  return (
    <>
      <PageHead
        title="Overview"
        subtitle="Per-key usage across the gateway: requests, tokens and spend for the current period."
      />
      {!adminKey && <NeedsKey openSettings={openSettings} />}
      <ErrorNotice error={error} />
      {adminKey && (
        <>
          <div className="cards">
            {loading && data === null ? (
              [0, 1, 2, 3].map((i) => (
                <Card key={i}>
                  <Skeleton width={72} height={11} />
                  <div style={{ marginTop: 10 }}>
                    <Skeleton width={120} height={22} />
                  </div>
                </Card>
              ))
            ) : (
              <>
                <Stat
                  label="Requests"
                  value={totals.requests}
                  delay={0}
                  spark={
                    sparkValues.length > 0 ? (
                      <Sparkline
                        values={sparkValues}
                        label={`Requests per hour over the last ${SPARK_HOURS} hours`}
                      />
                    ) : undefined
                  }
                />
                <Stat label="Tokens" value={totals.tokens} delay={60} />
                <Stat label="Spend" value={totals.spend} decimals={2} prefix="$" delay={120} />
                <Stat label="Active keys" value={usage.length} delay={180} />
              </>
            )}
          </div>

          <div className="ov-grid">
            <Panel>
              <PanelHead
                title="Activity"
                actions={
                  anyLive ? (
                    <span className="ov-live">
                      <span className="ov-dot live" aria-hidden="true" />
                      live
                    </span>
                  ) : undefined
                }
              />
              {activity.error && <ErrorNotice error={activity.error} />}
              {activity.loading ? (
                <div style={{ padding: "14px 18px" }}>
                  <Skeleton lines={5} height={13} />
                </div>
              ) : feed.length === 0 && !activity.error ? (
                <EmptyState
                  title="No recent activity"
                  description="Agent runs and guardrail verdicts will stream in here as they happen."
                />
              ) : (
                <ul className="ov-feed">
                  <AnimatePresence initial={false}>
                    {feed.map((e) => {
                      const live = isInflight(e, now);
                      const err = e.kind === "guardrail_block" || e.status >= 400;
                      return (
                        <motion.li
                          key={feedEntryId(e)}
                          className="ov-feed-item"
                          layout={!reduced}
                          variants={reduced ? staggerItemReduced : staggerItem}
                          initial="hidden"
                          animate="show"
                          exit={{ opacity: 0, transition: transitionFast }}
                        >
                          <span className={dotClass(e, live)} aria-hidden="true" />
                          <div className="ov-feed-main">
                            <div className="ov-feed-title">
                              <Badge variant={e.kind}>{e.kind.replace("_", " ")}</Badge>
                              <span className="mono">{e.key_name}</span>
                            </div>
                            <div className="ov-feed-sub mono">
                              {e.model} · {formatInt(e.input_tokens + e.output_tokens)} tok ·{" "}
                              {formatTimestamp(e.ts)}
                            </div>
                          </div>
                          <span className={`ov-feed-cost${err ? " err" : ""}`}>
                            {formatUSD(e.cost_usd)}
                          </span>
                        </motion.li>
                      );
                    })}
                  </AnimatePresence>
                </ul>
              )}
            </Panel>

            <Panel>
              <PanelHead title="Budgets" />
              {keysLoading && keysData === null ? (
                <div style={{ padding: "14px 18px" }}>
                  <Skeleton lines={3} height={13} />
                </div>
              ) : meters.length === 0 ? (
                <EmptyState
                  title="No budgets set"
                  description="Monthly budgets are set per key on the Keys page."
                />
              ) : (
                <div className="ov-meters">
                  {meters.map((m, i) => (
                    <div className="ov-meter-row" key={m.name}>
                      <div className="ov-meter-head">
                        <span className="ov-meter-name">{m.name}</span>
                        <span className="ov-meter-nums">
                          {formatUSD(m.spend)} / {formatUSD(m.budget)}
                        </span>
                      </div>
                      <div className="ov-meter-track">
                        <motion.div
                          className={`ov-meter-fill ${meterTone(m.fraction)}`.trim()}
                          initial={reduced ? false : { width: 0 }}
                          animate={{ width: `${m.fraction * 100}%` }}
                          transition={{ duration: 0.6, ease: EASE, delay: 0.1 + i * 0.06 }}
                        />
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </Panel>
          </div>

          <Panel>
            <PanelHead title="Usage by key" />
            {usage.length === 0 && !loading ? (
              <EmptyState
                title="No usage recorded yet"
                description="Requests made through the gateway will appear here, grouped by key."
              />
            ) : (
              <Table>
                <thead>
                  <tr>
                    <th>Key</th>
                    <th className="num">Requests</th>
                    <th className="num">Input tokens</th>
                    <th className="num">Output tokens</th>
                    <th className="num">Spend</th>
                  </tr>
                </thead>
                {loading && data === null ? (
                  <tbody>
                    {[0, 1, 2, 3].map((i) => (
                      <tr key={i}>
                        <td colSpan={5}>
                          <Skeleton height={13} />
                        </td>
                      </tr>
                    ))}
                  </tbody>
                ) : (
                  <Tbody staggerKey={usage.length}>
                    {usage.map((u) => (
                      <Tr key={u.name}>
                        <td className="mono">{u.name}</td>
                        <td className="num">{formatInt(u.requests)}</td>
                        <td className="num">{formatInt(u.input_tokens)}</td>
                        <td className="num">{formatInt(u.output_tokens)}</td>
                        <td className="num">{formatUSD(u.spend_usd)}</td>
                      </Tr>
                    ))}
                  </Tbody>
                )}
              </Table>
            )}
          </Panel>
        </>
      )}
    </>
  );
}
