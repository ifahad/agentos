import { motion, useReducedMotion } from "framer-motion";
import { useState, type CSSProperties } from "react";
import type { PageProps } from "../App";
import { CopyButton, ErrorNotice, NeedsKey, PageHead, errorMessage, useLoad } from "../components/common";
import { apiFetch, gatewayAdminRequest } from "../lib/api";
import { budgetFraction, formatUSD } from "../lib/format";
import { can } from "../lib/rbac";
import type { CreatedKey, KeyInfo } from "../lib/types";
import {
  Button,
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

export function Keys({ adminKey, role, openSettings }: PageProps) {
  const canCreate = can(role, "key.create");
  const toast = useToast();
  const reduced = useReducedMotion();
  const { data, error, loading, reload } = useLoad(
    () =>
      adminKey
        ? apiFetch<KeyInfo[]>(gatewayAdminRequest("/admin/keys", adminKey))
        : Promise.resolve<KeyInfo[]>([]),
    [adminKey],
  );

  const [name, setName] = useState("");
  const [budget, setBudget] = useState("25");
  const [creating, setCreating] = useState(false);
  const [created, setCreated] = useState<CreatedKey | null>(null);

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
      toast.success(`Key "${res.name}" created — store the secret now.`);
    } catch (err) {
      toast.error(errorMessage(err));
    } finally {
      setCreating(false);
    }
  };

  const keys = data ?? [];

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
            <a
              href="#dismiss"
              onClick={(e) => {
                e.preventDefault();
                setCreated(null);
              }}
            >
              Dismiss
            </a>
          </div>
        </motion.div>
      )}

      {adminKey && (
        <>
          {canCreate && (
            <Panel>
              <PanelHead title="Create key" />
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
                <Button variant="primary" onClick={() => void create()} disabled={creating}>
                  {creating ? "Creating…" : "Create key"}
                </Button>
              </div>
            </Panel>
          )}

          <Panel>
            <PanelHead title="Existing keys" />
            <Table>
              <thead>
                <tr>
                  <th>Name</th>
                  <th className="num">Monthly budget</th>
                  <th className="num">Spend</th>
                  <th>Budget used</th>
                </tr>
              </thead>
              <Tbody staggerKey={keys.length}>
                {loading && keys.length === 0 &&
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
                {keys.map((k, i) => {
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
                {keys.length === 0 && !loading && (
                  <Tr animate={false}>
                    <td colSpan={4} className="empty">
                      No keys yet — create one above.
                    </td>
                  </Tr>
                )}
              </Tbody>
            </Table>
          </Panel>
        </>
      )}
    </>
  );
}
