import type { PageProps } from "../App";
import { ErrorNotice, NeedsKey, PageHead, useLoad } from "../components/common";
import { apiFetch, gatewayAdminRequest } from "../lib/api";
import { formatInt, formatLatency, formatTimestamp, formatUSD } from "../lib/format";
import type { AuditEntry } from "../lib/types";
import { Badge, Button, Panel, PanelHead, Table, Tbody, Tr } from "../ui";
import "./Audit.css";

const GUARDRAIL_KINDS = new Set<AuditEntry["kind"]>(["guardrail_flag", "guardrail_block"]);

export function Audit({ adminKey, openSettings }: PageProps) {
  const { data, error, loading, reload } = useLoad(
    () =>
      adminKey
        ? apiFetch<AuditEntry[]>(gatewayAdminRequest("/admin/audit?limit=100", adminKey))
        : Promise.resolve<AuditEntry[]>([]),
    [adminKey],
  );

  const entries = data ?? [];

  return (
    <>
      <PageHead
        title="Audit"
        subtitle="Latest 100 gateway events, newest first — chat and embedding calls plus guardrail verdicts."
      />
      {!adminKey && <NeedsKey openSettings={openSettings} />}
      <ErrorNotice error={error} />
      {adminKey && (
        <Panel>
          <PanelHead
            title="Events"
            actions={
              <Button small onClick={reload} disabled={loading}>
                {loading ? "Loading…" : "Refresh"}
              </Button>
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
            <Tbody staggerKey={`${entries[0]?.ts ?? "none"}-${entries.length}`}>
              {entries.map((e, i) => (
                <Tr key={`${e.ts}-${i}`}>
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
              {entries.length === 0 && !loading && (
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
