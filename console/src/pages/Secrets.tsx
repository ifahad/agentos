import { useState } from "react";
import type { PageProps } from "../App";
import { ErrorNotice, ForbiddenNotice, NeedsKey, PageHead, errorMessage, useLoad } from "../components/common";
import { ApiError, apiFetch, gatewayAdminRequest, reloadSecretsRequest } from "../lib/api";
import { can } from "../lib/rbac";
import type { SecretStatus } from "../lib/types";
import { Badge, Button, EmptyState, Panel, PanelHead, Skeleton, Table, Tbody, Tr, useToast } from "../ui";

export function Secrets({ adminKey, role, openSettings }: PageProps) {
  const allowed = can(role, "secret.view");
  const toast = useToast();

  const { data, error, loading, reload } = useLoad(
    () =>
      adminKey && allowed
        ? apiFetch<SecretStatus[]>(gatewayAdminRequest("/admin/secrets/status", adminKey))
        : Promise.resolve<SecretStatus[]>([]),
    [adminKey, allowed],
  );

  const [reloading, setReloading] = useState(false);
  const [forbidden, setForbidden] = useState(false);

  const reloadSecrets = async () => {
    setReloading(true);
    setForbidden(false);
    try {
      // The endpoint returns the fresh status array; refetch to reflect it.
      await apiFetch<SecretStatus[]>(reloadSecretsRequest(adminKey));
      reload();
      toast.success("Secrets reloaded.");
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) {
        setForbidden(true);
      } else {
        toast.error(errorMessage(err));
      }
    } finally {
      setReloading(false);
    }
  };

  const secrets = data ?? [];

  return (
    <>
      <PageHead
        title="Secrets"
        subtitle="Provider secrets resolved by the gateway — presence and backend only. Values are never exposed. Root admin only."
      />
      {!adminKey && <NeedsKey openSettings={openSettings} />}
      {adminKey && !allowed && <ForbiddenNotice message="Secret status is visible to the root admin only." />}
      <ErrorNotice error={error} />

      {(!adminKey || allowed) && (
        <Panel>
          <PanelHead
            title="Secret status"
            actions={
              <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                <Button
                  small
                  icon="refresh"
                  onClick={() => void reloadSecrets()}
                  disabled={reloading || loading}
                >
                  {reloading ? "Reloading…" : "Reload secrets"}
                </Button>
                <Button small icon="refresh" onClick={reload} disabled={loading}>
                  {loading ? "Loading…" : "Refresh"}
                </Button>
              </div>
            }
          />
          {forbidden && <ForbiddenNotice message="Reloading secrets is available to the root admin only." />}
          {!adminKey ? (
            <EmptyState
              title="No admin key configured"
              description="Provider secrets resolved by the gateway — presence and backend only — appear here once the console can reach the admin API."
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
                  <th>Name</th>
                  <th>Present</th>
                  <th>Source</th>
                </tr>
              </thead>
              <Tbody staggerKey={secrets.length}>
                {loading && secrets.length === 0 &&
                  [0, 1, 2].map((i) => (
                    <Tr animate={false} key={`skeleton-${i}`}>
                      <td>
                        <Skeleton width={200} />
                      </td>
                      <td>
                        <Skeleton width={64} height={18} style={{ borderRadius: 999 }} />
                      </td>
                      <td>
                        <Skeleton width={72} height={18} style={{ borderRadius: 999 }} />
                      </td>
                    </Tr>
                  ))}
                {secrets.map((s) => (
                  <Tr key={s.name}>
                    <td className="mono">{s.name}</td>
                    <td>
                      <Badge variant={s.present ? "pass" : "fail"}>
                        {s.present ? "present" : "missing"}
                      </Badge>
                    </td>
                    <td>
                      <Badge>{s.source}</Badge>
                    </td>
                  </Tr>
                ))}
                {secrets.length === 0 && !loading && (
                  <Tr animate={false}>
                    <td colSpan={3} className="empty">
                      No secrets reported.
                    </td>
                  </Tr>
                )}
              </Tbody>
            </Table>
          )}
        </Panel>
      )}
    </>
  );
}
