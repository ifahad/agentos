import type { PageProps } from "../App";
import { ErrorNotice, NeedsKey, PageHead, useLoad } from "../components/common";
import { apiFetch, gatewayAdminRequest } from "../lib/api";
import { formatInt, formatLatency, formatTimestamp, formatUSD } from "../lib/format";
import type { AuditEntry } from "../lib/types";

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
        <div className="panel">
          <div className="panel-head">
            <h2>Events</h2>
            <button className="btn small" onClick={reload} disabled={loading}>
              {loading ? "Loading…" : "Refresh"}
            </button>
          </div>
          <div className="table-wrap">
            <table>
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
              <tbody>
                {entries.map((e, i) => (
                  <tr key={`${e.ts}-${i}`}>
                    <td className="dim mono">{formatTimestamp(e.ts)}</td>
                    <td className="mono">{e.key_name}</td>
                    <td className="mono">{e.model}</td>
                    <td>
                      <span className={`badge ${e.kind}`}>{e.kind}</span>
                    </td>
                    <td className="num">
                      {formatInt(e.input_tokens)} / {formatInt(e.output_tokens)}
                    </td>
                    <td className="num">{formatUSD(e.cost_usd)}</td>
                    <td className="num">{formatLatency(e.latency_ms)}</td>
                    <td className={`num ${e.status < 400 ? "status-ok" : "status-err"}`}>
                      {e.status}
                    </td>
                  </tr>
                ))}
                {entries.length === 0 && !loading && (
                  <tr>
                    <td colSpan={8} className="empty">
                      No audit events yet.
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </>
  );
}
