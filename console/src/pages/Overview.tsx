import { useEffect, useMemo, useState } from "react";
import { motion, useReducedMotion } from "framer-motion";
import type { PageProps } from "../App";
import { ErrorNotice, NeedsKey, PageHead } from "../components/common";
import { Freshness } from "../components/Freshness";
import { LiveList } from "../components/LiveList";
import { useLiveResource } from "../hooks/useLiveResource";
import { apiFetch, gatewayAdminRequest } from "../lib/api";
import { formatInt, formatTimestamp, formatUSD } from "../lib/format";
import type { AuditEntry, KeyInfo, KeyUsage } from "../lib/types";
import {
  Badge,
  Button,
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
} from "../ui";
import { Sparkline, SpendBreakdown, breakdownFromRows, normalizeSeries, seriesFromEvents } from "../charts";
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

export function Overview({ adminKey, openSettings, navigate }: PageProps) {
  const reduced = useReducedMotion();

  // Usage totals — was useLoad (fetch-once); now live.
  const usageRes = useLiveResource<KeyUsage[]>(
    `admin/usage#${adminKey}`,
    () => apiFetch<KeyUsage[]>(gatewayAdminRequest("/admin/usage", adminKey)),
    { enabled: Boolean(adminKey), cadence: 5000 },
  );
  const usage = usageRes.data ?? [];
  const usageLoading = usageRes.status === "loading";

  // Keys/budgets — degrade to empty on a role that can't list keys.
  const keysRes = useLiveResource<KeyInfo[]>(
    `admin/keys#${adminKey}#ov`,
    () =>
      apiFetch<KeyInfo[]>(gatewayAdminRequest("/admin/keys", adminKey)).catch(
        () => [] as KeyInfo[],
      ),
    { enabled: Boolean(adminKey), cadence: 5000 },
  );
  const keysLoading = keysRes.status === "loading";

  // Activity feed — the shared audit resource; keep the client-side merge/cap.
  const auditRes = useLiveResource<AuditEntry[]>(
    `admin/audit?limit=${AUDIT_LIMIT}#${adminKey}`,
    () => apiFetch<AuditEntry[]>(gatewayAdminRequest(`/admin/audit?limit=${AUDIT_LIMIT}`, adminKey)),
    { enabled: Boolean(adminKey), cadence: POLL_MS },
  );
  const [feedEntries, setFeedEntries] = useState<AuditEntry[]>([]);
  useEffect(() => {
    if (auditRes.data) setFeedEntries((cur) => mergeFeedEntries(cur, auditRes.data!, AUDIT_LIMIT));
  }, [auditRes.data]);
  const activityLoading = auditRes.status === "loading";

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
    if (feedEntries.length === 0) return [];
    const series = seriesFromEvents(
      feedEntries.map((e) => ({ timestamp: e.ts })),
      { bucket: "hour" },
    );
    return normalizeSeries(
      series.map((p) => p.value),
      SPARK_HOURS,
    );
  }, [feedEntries]);

  const feed = feedEntries.slice(0, FEED_DISPLAY);
  const now = Date.now();
  const anyLive = feed.some((e) => isInflight(e, now));

  const meters = budgetMeters(keysRes.data ?? []);

  return (
    <>
      <PageHead
        title="Overview"
        subtitle="Per-key usage across the gateway: requests, tokens and spend for the current period."
      />
      {!adminKey && <NeedsKey openSettings={openSettings} />}
      <ErrorNotice error={usageRes.error} />
      <div className="cards">
        {usageLoading ? (
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
              <span className="head-group">
                {anyLive && (
                  <span className="ov-live">
                    <span className="ov-dot live" aria-hidden="true" />
                    live
                  </span>
                )}
                <Freshness updatedAt={auditRes.updatedAt} />
              </span>
            }
          />
          {auditRes.error && <ErrorNotice error={auditRes.error} />}
          {!adminKey ? (
            <EmptyState
              title="No admin key configured"
              description="Agent runs and guardrail verdicts stream in here once the console can reach the admin API."
              action={
                <Button variant="primary" onClick={openSettings}>
                  Open settings
                </Button>
              }
            />
          ) : activityLoading ? (
            <div style={{ padding: "14px 18px" }}>
              <Skeleton lines={5} height={13} />
            </div>
          ) : feed.length === 0 && !auditRes.error ? (
            <EmptyState
              title="No recent activity"
              description="Agent runs and guardrail verdicts will stream in here as they happen."
            />
          ) : (
            <LiveList
              className="ov-feed"
              items={feed}
              getKey={feedEntryId}
              renderItem={(e) => {
                const live = isInflight(e, now);
                const err = e.kind === "guardrail_block" || e.status >= 400;
                return (
                  <>
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
                  </>
                );
              }}
            />
          )}
        </Panel>

        <Panel>
          <PanelHead title="Budgets" />
          {!adminKey ? (
            <EmptyState
              title="No admin key configured"
              description="Monthly budgets, set per key on the Keys page, appear here once the console can reach the admin API."
              action={
                <Button variant="primary" onClick={openSettings}>
                  Open settings
                </Button>
              }
            />
          ) : keysLoading ? (
            <div style={{ padding: "14px 18px" }}>
              <Skeleton lines={3} height={13} />
            </div>
          ) : meters.length === 0 ? (
            <EmptyState
              title="No budgets set"
              description="Monthly budgets are set per key on the Keys page."
              action={
                <Button small onClick={() => navigate("/keys")}>
                  Open Keys
                </Button>
              }
            />
          ) : (
            <div className="ov-meters">
              {meters.map((m, i) => (
                <div className="ov-meter-row" key={m.id}>
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
                      /* Not a preset: a one-time budget-meter draw-in, slower
                         (600ms) than any entrance preset and staggered by
                         row index rather than STAGGER, so each bar reads
                         as filling to its own measured fraction. */
                      transition={{ duration: 0.6, ease: EASE, delay: 0.1 + i * 0.06 }}
                    />
                  </div>
                </div>
              ))}
            </div>
          )}
        </Panel>

        <Panel>
          <PanelHead title="Spend by key" />
          <SpendBreakdown
            rows={breakdownFromRows(usage, { label: (u) => u.name, value: (u) => u.spend_usd })}
            label="Spend by key"
            formatValue={formatUSD}
          />
        </Panel>
      </div>

      <Panel>
        <PanelHead title="Usage by key" />
        {!adminKey ? (
          <EmptyState
            title="No admin key configured"
            description="Requests made through the gateway will appear here, grouped by key, once the console can reach the admin API."
            action={
              <Button variant="primary" onClick={openSettings}>
                Open settings
              </Button>
            }
          />
        ) : usage.length === 0 && !usageLoading ? (
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
            {usageLoading ? (
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
                {/* Key names are not unique and the gateway exposes no id,
                    so rows are identified by name plus position. */}
                {usage.map((u, i) => (
                  <Tr key={`${u.name}#${i}`}>
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
  );
}
