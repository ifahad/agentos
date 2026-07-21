import type { PageProps } from "../App";
import { ErrorNotice, NeedsKey, PageHead, useLoad } from "../components/common";
import { apiFetch, gatewayAdminRequest } from "../lib/api";
import { formatInt, formatUSD } from "../lib/format";
import type { KeyUsage } from "../lib/types";

export function Overview({ adminKey, openSettings }: PageProps) {
  const { data, error, loading } = useLoad(
    () =>
      adminKey
        ? apiFetch<KeyUsage[]>(gatewayAdminRequest("/admin/usage", adminKey))
        : Promise.resolve<KeyUsage[]>([]),
    [adminKey],
  );

  const usage = data ?? [];
  const totals = usage.reduce(
    (acc, u) => ({
      requests: acc.requests + u.requests,
      tokens: acc.tokens + u.input_tokens + u.output_tokens,
      spend: acc.spend + u.spend_usd,
    }),
    { requests: 0, tokens: 0, spend: 0 },
  );

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
            <div className="card">
              <div className="label">Requests</div>
              <div className="value">{formatInt(totals.requests)}</div>
            </div>
            <div className="card">
              <div className="label">Tokens</div>
              <div className="value">{formatInt(totals.tokens)}</div>
            </div>
            <div className="card">
              <div className="label">Spend</div>
              <div className="value">{formatUSD(totals.spend)}</div>
            </div>
            <div className="card">
              <div className="label">Active keys</div>
              <div className="value">{formatInt(usage.length)}</div>
            </div>
          </div>
          <div className="panel">
            <div className="panel-head">
              <h2>Usage by key</h2>
              {loading && <span className="spin">loading…</span>}
            </div>
            <div className="table-wrap">
              <table>
                <thead>
                  <tr>
                    <th>Key</th>
                    <th className="num">Requests</th>
                    <th className="num">Input tokens</th>
                    <th className="num">Output tokens</th>
                    <th className="num">Spend</th>
                  </tr>
                </thead>
                <tbody>
                  {usage.map((u) => (
                    <tr key={u.name}>
                      <td className="mono">{u.name}</td>
                      <td className="num">{formatInt(u.requests)}</td>
                      <td className="num">{formatInt(u.input_tokens)}</td>
                      <td className="num">{formatInt(u.output_tokens)}</td>
                      <td className="num">{formatUSD(u.spend_usd)}</td>
                    </tr>
                  ))}
                  {usage.length === 0 && !loading && (
                    <tr>
                      <td colSpan={5} className="empty">
                        No usage recorded yet.
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>
          </div>
        </>
      )}
    </>
  );
}
