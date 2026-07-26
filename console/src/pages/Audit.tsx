import { useEffect, useMemo, useState } from "react";
import type { PageProps } from "../App";
import { ErrorNotice, NeedsKey, PageHead } from "../components/common";
import { Freshness } from "../components/Freshness";
import { SortableTh } from "../components/SortableTh";
import { TableToolbar } from "../components/TableToolbar";
import { seriesFromEvents, UsageChart } from "../charts";
import { useLiveResource } from "../hooks/useLiveResource";
import { useTableView } from "../hooks/useTableView";
import { apiFetch, gatewayAdminRequest } from "../lib/api";
import { downloadBlob, toCSV, toJSON } from "../lib/export";
import type { Column } from "../lib/export";
import { formatInt, formatLatency, formatTimestamp, formatUSD } from "../lib/format";
import type { AuditEntry } from "../lib/types";
import { Badge, Button, Panel, PanelHead, Table, Tbody, Tr } from "../ui";
import { feedEntryId, mergeFeedEntries } from "./overviewFeed";
import "./Audit.css";

const GUARDRAIL_KINDS = new Set<AuditEntry["kind"]>(["guardrail_flag", "guardrail_block"]);
const KIND_OPTIONS = ["chat", "embeddings", "guardrail_flag", "guardrail_block"] as const;

const AUDIT_COLUMNS: Column<AuditEntry>[] = [
  { header: "Time", value: (e) => formatTimestamp(e.ts) },
  { header: "Key", value: (e) => e.key_name },
  { header: "Model", value: (e) => e.model },
  { header: "Kind", value: (e) => e.kind },
  { header: "Tokens in", value: (e) => e.input_tokens },
  { header: "Tokens out", value: (e) => e.output_tokens },
  { header: "Cost", value: (e) => e.cost_usd },
  { header: "Latency", value: (e) => e.latency_ms },
  { header: "Status", value: (e) => e.status },
];

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

  // Search/filter/sort/paginate over the live feed — sort null preserves the
  // merge's newest-first order; picking a sort intentionally overrides it.
  const t = useTableView(rows, { searchFields: ["key_name", "model"], pageSize: 25 });

  // Daily spend, bucketed from the same feed the table renders.
  const series = useMemo(
    () => seriesFromEvents(rows.map((e) => ({ timestamp: e.ts, value: e.cost_usd })), { bucket: "day" }),
    [rows],
  );

  function onExport(format: "csv" | "json") {
    if (format === "csv") {
      downloadBlob("audit.csv", "text/csv;charset=utf-8", toCSV(t.view.filtered, AUDIT_COLUMNS));
    } else {
      downloadBlob("audit.json", "application/json", toJSON(t.view.filtered, AUDIT_COLUMNS));
    }
  }

  return (
    <>
      <PageHead
        title="Audit"
        subtitle="Latest 100 gateway events, newest first — chat and embedding calls plus guardrail verdicts."
      />
      {!adminKey && <NeedsKey openSettings={openSettings} />}
      <ErrorNotice error={res.error} />
      {adminKey && (
        <>
          {rows.length > 0 && (
            <Panel>
              <PanelHead title="Spend" />
              <UsageChart
                values={series.map((p) => p.value)}
                label="Spend per day"
                xLabels={series.map((p) => new Date(p.t).toISOString().slice(5, 10))}
                formatValue={formatUSD}
              />
            </Panel>
          )}
          <Panel>
            <PanelHead
              title="Events"
              actions={
                <span className="head-group">
                  <TableToolbar
                    query={t.query}
                    onQuery={t.setQuery}
                    facets={[
                      {
                        key: "kind",
                        label: "Kind",
                        options: KIND_OPTIONS,
                        value: t.facets.kind ?? "",
                        onChange: (v) => t.setFacet("kind", v),
                      },
                    ]}
                    onExport={onExport}
                  />
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
                  <SortableTh<AuditEntry>
                    label="Key"
                    sortKey="key_name"
                    active={t.sort?.key === "key_name"}
                    dir={t.sort?.dir ?? "asc"}
                    onSort={t.toggleSort}
                  />
                  <SortableTh<AuditEntry>
                    label="Model"
                    sortKey="model"
                    active={t.sort?.key === "model"}
                    dir={t.sort?.dir ?? "asc"}
                    onSort={t.toggleSort}
                  />
                  <th>Kind</th>
                  <SortableTh<AuditEntry>
                    className="num"
                    label="Tokens in/out"
                    sortKey="input_tokens"
                    active={t.sort?.key === "input_tokens"}
                    dir={t.sort?.dir ?? "asc"}
                    numeric
                    onSort={t.toggleSort}
                  />
                  <SortableTh<AuditEntry>
                    className="num"
                    label="Cost"
                    sortKey="cost_usd"
                    active={t.sort?.key === "cost_usd"}
                    dir={t.sort?.dir ?? "asc"}
                    numeric
                    onSort={t.toggleSort}
                  />
                  <SortableTh<AuditEntry>
                    className="num"
                    label="Latency"
                    sortKey="latency_ms"
                    active={t.sort?.key === "latency_ms"}
                    dir={t.sort?.dir ?? "asc"}
                    numeric
                    onSort={t.toggleSort}
                  />
                  <SortableTh<AuditEntry>
                    className="num"
                    label="Status"
                    sortKey="status"
                    active={t.sort?.key === "status"}
                    dir={t.sort?.dir ?? "asc"}
                    numeric
                    onSort={t.toggleSort}
                  />
                </tr>
              </thead>
              <Tbody
                staggerKey={`${rows[0] ? feedEntryId(rows[0]) : "none"}-${rows.length}-${
                  t.sort ? `${String(t.sort.key)}:${t.sort.dir}` : "none"
                }`}
              >
                {t.view.rows.map((e) => (
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
                {t.view.rows.length === 0 && (
                  <Tr animate={false}>
                    <td colSpan={8} className="empty">
                      {rows.length === 0 ? "No audit events yet." : "No events match your search."}
                    </td>
                  </Tr>
                )}
              </Tbody>
            </Table>
            {t.view.pageCount > 1 && (
              <div className="pager">
                <Button small disabled={t.view.page === 0} onClick={() => t.setPage(t.view.page - 1)}>
                  Prev
                </Button>
                <span className="pager-info">
                  page {t.view.page + 1} of {t.view.pageCount}
                </span>
                <Button
                  small
                  disabled={t.view.page >= t.view.pageCount - 1}
                  onClick={() => t.setPage(t.view.page + 1)}
                >
                  Next
                </Button>
              </div>
            )}
          </Panel>
        </>
      )}
    </>
  );
}
