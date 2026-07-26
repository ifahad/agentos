import { useEffect, useState } from "react";
import type { PageProps } from "../App";
import { ErrorNotice, NeedsKey, PageHead } from "../components/common";
import { Freshness } from "../components/Freshness";
import { useLiveResource } from "../hooks/useLiveResource";
import { apiFetch, gatewayAdminRequest } from "../lib/api";
import { formatInt, formatLatency, formatTimestamp, formatUSD } from "../lib/format";
import type { AuditEntry } from "../lib/types";
import { Badge, Button, Panel, PanelHead, Table, Tbody, Tr } from "../ui";
import { feedEntryId, mergeFeedEntries } from "./overviewFeed";
import "./Audit.css";

const GUARDRAIL_KINDS = new Set<AuditEntry["kind"]>(["guardrail_flag", "guardrail_block"]);

export function Audit({ adminKey, openSettings }: PageProps) {
  // Same audit resource Overview/Chain subscribe to (identical key + cadence),
  // so the registry dedupes onto a single shared poll.
  const res = useLiveResource<AuditEntry[]>(
    `admin/audit?limit=100#${adminKey}`,
    () => apiFetch<AuditEntry[]>(gatewayAdminRequest("/admin/audit?limit=100", adminKey)),
    { enabled: Boolean(adminKey), cadence: 5000 },
  );

  const [rows, setRows] = useState<AuditEntry[]>([]);
  const [rowsAt, setRowsAt] = useState<number | null>(null);
  const [paused, setPaused] = useState(false);

  useEffect(() => {
    if (paused || !res.data) return;
    setRows((cur) => mergeFeedEntries(cur, res.data!, 100));
    setRowsAt(res.updatedAt);
  }, [res.data, res.updatedAt, paused]);

  return (
    <>
      <PageHead
        title="Audit"
        subtitle="Latest 100 gateway events, newest first — chat and embedding calls plus guardrail verdicts."
      />
      {!adminKey && <NeedsKey openSettings={openSettings} />}
      <ErrorNotice error={res.error} />
      {adminKey && (
        <Panel>
          <PanelHead
            title="Events"
            actions={
              <span className="head-group">
                <Button small onClick={() => setPaused((p) => !p)}>
                  {paused ? "Paused" : "Live"}
                </Button>
                <Freshness updatedAt={rowsAt} />
                <Button small icon="refresh" onClick={res.reload}>
                  Refresh
                </Button>
              </span>
            }
          />
          <Table>
            <thead>
              <tr>
                <th>Time</th>
                <th>Key</th>
                <th>Model</th>
                <th>Kind</th>
                <th className="num">Tokens in/out</th>
                <th className="num">Cost</th>
                <th className="num">Latency</th>
                <th className="num">Status</th>
              </tr>
            </thead>
            <Tbody staggerKey={`${rows[0] ? feedEntryId(rows[0]) : "none"}-${rows.length}`}>
              {rows.map((e) => (
                <Tr key={feedEntryId(e)}>
                  <td className="dim mono">{formatTimestamp(e.ts)}</td>
                  <td className="mono">{e.key_name}</td>
                  <td className="mono">{e.model}</td>
                  <td>
                    <span className={GUARDRAIL_KINDS.has(e.kind) ? "audit-badge-flash" : undefined}>
                      <Badge variant={e.kind}>{e.kind.replace("guardrail_", "guardrail ")}</Badge>
                    </span>
                  </td>
                  <td className="num">
                    {formatInt(e.input_tokens)} / {formatInt(e.output_tokens)}
                  </td>
                  <td className="num">{formatUSD(e.cost_usd)}</td>
                  <td className="num">{formatLatency(e.latency_ms)}</td>
                  <td className={`num ${e.status < 400 ? "status-ok" : "status-err"}`}>
                    {e.status}
                  </td>
                </Tr>
              ))}
              {rows.length === 0 && (
                <Tr animate={false}>
                  <td colSpan={8} className="empty">
                    No audit events yet.
                  </td>
                </Tr>
              )}
            </Tbody>
          </Table>
        </Panel>
      )}
    </>
  );
}
