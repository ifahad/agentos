import { motion, useReducedMotion } from "framer-motion";
import { useState, type CSSProperties } from "react";
import type { PageProps } from "../App";
import { CopyButton, ErrorNotice, NeedsKey, PageHead, errorMessage } from "../components/common";
import { Freshness } from "../components/Freshness";
import { SortableTh } from "../components/SortableTh";
import { TableToolbar } from "../components/TableToolbar";
import { useLiveResource } from "../hooks/useLiveResource";
import { useTableView } from "../hooks/useTableView";
import { apiFetch, gatewayAdminRequest } from "../lib/api";
import { downloadBlob, toCSV, toJSON } from "../lib/export";
import type { Column } from "../lib/export";
import { budgetFraction, formatUSD } from "../lib/format";
import { can } from "../lib/rbac";
import { keysSummary } from "../lib/summaries";
import type { CreatedKey, KeyInfo } from "../lib/types";
import {
  Button,
  Disclosure,
  EmptyState,
  Input,
  Panel,
  PanelHead,
  Skeleton,
  Table,
  Tbody,
  Tr,
  fadeRise,
  fadeRiseReduced,
  useToast,
} from "../ui";
import "./Keys.css";

// Hoisted to module scope so useTableView's memo dependency is stable.
const SEARCH_FIELDS = ["name"] as const;

const KEYS_COLUMNS: Column<KeyInfo>[] = [
  { header: "Name", value: (k) => k.name },
  { header: "Monthly budget", value: (k) => formatUSD(k.monthly_budget_usd) },
  { header: "Spend", value: (k) => formatUSD(k.spend_usd) },
  {
    header: "Budget used",
    value: (k) => `${Math.round(budgetFraction(k.spend_usd, k.monthly_budget_usd) * 100)}%`,
  },
];

export function Keys({ adminKey, role, openSettings }: PageProps) {
  const canCreate = can(role, "key.create");
  const toast = useToast();
  const reduced = useReducedMotion();
  const { data, error, status, updatedAt, reload } = useLiveResource<KeyInfo[]>(
    `admin/keys#${adminKey}`,
    () => apiFetch<KeyInfo[]>(gatewayAdminRequest("/admin/keys", adminKey)),
    { enabled: Boolean(adminKey), cadence: 5000 },
  );
  const keysLoading = status === "loading";
  const keys = data ?? [];
  const t = useTableView(keys, { searchFields: SEARCH_FIELDS, initialSort: { key: "name", dir: "asc" } });

  const [name, setName] = useState("");
  const [budget, setBudget] = useState("25");
  const [creating, setCreating] = useState(false);
  const [created, setCreated] = useState<CreatedKey | null>(null);
  const [createOpen, setCreateOpen] = useState(false);

  const create = async () => {
    const budgetNum = Number(budget);
    if (!name.trim() || !Number.isFinite(budgetNum) || budgetNum < 0) {
      toast.error("A name and a non-negative monthly budget are required.");
      return;
    }
    setCreating(true);
    try {
      const res = await apiFetch<CreatedKey>(
        gatewayAdminRequest("/admin/keys", adminKey, {
          name: name.trim(),
          monthly_budget_usd: budgetNum,
        }),
      );
      setCreated(res);
      setName("");
      reload();
      setCreateOpen(false);
      toast.success(`Key "${res.name}" created — store the secret now.`);
    } catch (err) {
      toast.error(errorMessage(err));
    } finally {
      setCreating(false);
    }
  };

  function onExport(format: "csv" | "json") {
    if (format === "csv") {
      downloadBlob("keys.csv", "text/csv;charset=utf-8", toCSV(t.view.filtered, KEYS_COLUMNS));
    } else {
      downloadBlob("keys.json", "application/json", toJSON(t.view.filtered, KEYS_COLUMNS));
    }
  }

  return (
    <>
      <PageHead
        title="Keys"
        subtitle="Virtual gateway keys with monthly budgets. Secrets are shown exactly once at creation."
      />
      {!adminKey && <NeedsKey openSettings={openSettings} />}
      <ErrorNotice error={error} />

      {created && (
        <motion.div
          className="secret-reveal"
          variants={reduced ? fadeRiseReduced : fadeRise}
          initial="hidden"
          animate="show"
        >
          <strong>
            Key <span className="mono">{created.name}</span> created
          </strong>
          <div className="secret">
            <code>{created.key}</code>
            <CopyButton text={created.key} />
          </div>
          <div className="muted">
            Store this secret now — it will not be shown again.{" "}
            <Button small iconOnly icon="close" aria-label="Dismiss" onClick={() => setCreated(null)} />
          </div>
        </motion.div>
      )}

      <Panel>
        <PanelHead
          title="Existing keys"
          summary={keysSummary(keys)}
          actions={
            <span className="head-group">
              <TableToolbar query={t.query} onQuery={t.setQuery} onExport={onExport} />
              <Freshness updatedAt={updatedAt} />
              {canCreate && (
                <Button
                  variant="primary"
                  icon="plus"
                  aria-expanded={createOpen}
                  aria-controls="keys-create"
                  onClick={() => setCreateOpen((v) => !v)}
                >
                  New key
                </Button>
              )}
            </span>
          }
        />
        {canCreate && (
          <Disclosure open={createOpen} onOpenChange={setCreateOpen} id="keys-create">
            <div className="panel-body">
              <div className="form-row">
                <Input
                  label="Name"
                  type="text"
                  value={name}
                  placeholder="team-analytics"
                  onChange={(e) => setName(e.target.value)}
                />
                <Input
                  label="Monthly budget (USD)"
                  type="number"
                  min="0"
                  step="1"
                  value={budget}
                  onChange={(e) => setBudget(e.target.value)}
                />
              </div>
              <Button variant="primary" icon="plus" onClick={() => void create()} disabled={creating}>
                {creating ? "Creating…" : "Create key"}
              </Button>
            </div>
          </Disclosure>
        )}
        {!adminKey ? (
          <EmptyState
            title="No admin key configured"
            description="Gateway keys and their monthly budgets appear here once the console can reach the admin API."
            action={
              <Button variant="primary" onClick={openSettings}>
                Open settings
              </Button>
            }
          />
        ) : (
          <Table>
            <thead>
              <tr>
                <SortableTh<KeyInfo>
                  label="Name"
                  sortKey="name"
                  active={t.sort?.key === "name"}
                  dir={t.sort?.dir ?? "asc"}
                  onSort={t.toggleSort}
                />
                <SortableTh<KeyInfo>
                  className="num"
                  label="Monthly budget"
                  sortKey="monthly_budget_usd"
                  active={t.sort?.key === "monthly_budget_usd"}
                  dir={t.sort?.dir ?? "asc"}
                  numeric
                  onSort={t.toggleSort}
                />
                <SortableTh<KeyInfo>
                  className="num"
                  label="Spend"
                  sortKey="spend_usd"
                  active={t.sort?.key === "spend_usd"}
                  dir={t.sort?.dir ?? "asc"}
                  numeric
                  onSort={t.toggleSort}
                />
                <th>Budget used</th>
              </tr>
            </thead>
            <Tbody staggerKey={`${t.view.rows.length}-${t.sort ? `${String(t.sort.key)}:${t.sort.dir}` : "none"}`}>
              {keysLoading && keys.length === 0 &&
                [0, 1, 2].map((i) => (
                  <Tr animate={false} key={`skeleton-${i}`}>
                    <td>
                      <Skeleton width={140} />
                    </td>
                    <td className="num">
                      <Skeleton width={56} style={{ marginLeft: "auto", display: "block" }} />
                    </td>
                    <td className="num">
                      <Skeleton width={56} style={{ marginLeft: "auto", display: "block" }} />
                    </td>
                    <td>
                      <Skeleton width={120} height={4} />
                    </td>
                  </Tr>
                ))}
              {/* Key names are not unique and the gateway exposes no id, so
                  rows are identified by name plus position. */}
              {t.view.rows.map((k, i) => {
                const frac = budgetFraction(k.spend_usd, k.monthly_budget_usd);
                const pct = Math.round(frac * 100);
                return (
                  <Tr key={`${k.name}#${i}`}>
                    <td className="mono">{k.name}</td>
                    <td className="num">{formatUSD(k.monthly_budget_usd)}</td>
                    <td className="num">{formatUSD(k.spend_usd)}</td>
                    <td>
                      <span className={`meter keys-meter${frac >= 0.9 ? " hot" : ""}`}>
                        <div style={{ "--w": `${pct}%` } as CSSProperties} />
                      </span>{" "}
                      <span className="dim muted">{pct}%</span>
                    </td>
                  </Tr>
                );
              })}
              {t.view.rows.length === 0 && !keysLoading && (
                <Tr animate={false}>
                  <td colSpan={4} className="empty">
                    {keys.length === 0 ? "No keys yet — create one above." : "No keys match your search."}
                  </td>
                </Tr>
              )}
            </Tbody>
          </Table>
        )}
      </Panel>
    </>
  );
}
