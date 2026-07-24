import { useEffect, useState } from "react";
import { createRoot } from "react-dom/client";
import "../../src/styles.css";
import {
  Badge,
  Button,
  Card,
  EmptyState,
  Input,
  Modal,
  Panel,
  PanelHead,
  Select,
  Skeleton,
  Stat,
  Table,
  Tabs,
  Tbody,
  Textarea,
  ToastProvider,
  Tr,
  useToast,
} from "../../src/ui";
import { Sparkline, SpendBreakdown, UsageChart } from "../../src/charts";
import { Sidebar } from "../../src/components/Sidebar";

function Core() {
  const [tab, setTab] = useState("active");
  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 22 }}>
      <section>
        <div className="pv-label">Buttons</div>
        <div style={{ display: "flex", gap: 10, flexWrap: "wrap" }}>
          <Button variant="primary">Create key</Button>
          <Button>Save changes</Button>
          <Button variant="danger">Revoke</Button>
          <Button small>Small</Button>
        </div>
      </section>
      <section>
        <div className="pv-label">Badges</div>
        <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
          <Badge variant="chat">chat</Badge>
          <Badge variant="embeddings">embeddings</Badge>
          <Badge variant="guardrail_flag">guardrail flag</Badge>
          <Badge variant="guardrail_block">guardrail block</Badge>
          <Badge variant="pass">pass</Badge>
          <Badge variant="failed_evals">failed evals</Badge>
          <Badge variant="inactive">inactive</Badge>
        </div>
      </section>
      <section>
        <div className="pv-label">Fields</div>
        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 14, maxWidth: 560 }}>
          <Input label="Key name" placeholder="production-agent" />
          <Select label="Mode">
            <option>chat</option>
            <option>embeddings</option>
          </Select>
        </div>
        <div style={{ maxWidth: 560, marginTop: 14 }}>
          <Textarea label="System prompt" placeholder="You are a helpful agent…" />
        </div>
      </section>
      <section>
        <div className="pv-label">Tabs — sliding pill</div>
        <Tabs
          tabs={[
            { id: "active", label: "Active" },
            { id: "revoked", label: "Revoked" },
            { id: "all", label: "All" },
          ]}
          active={tab}
          onChange={setTab}
        />
      </section>
    </div>
  );
}

function Data() {
  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 22 }}>
      <div className="cards">
        <Stat
          label="Requests today"
          value={12408}
          spark={<Sparkline values={[4, 6, 5, 9, 7, 11, 9, 13, 12, 16]} label="requests" />}
        />
        <Stat label="Spend today" value={41.2} prefix="$" decimals={2} />
        <Stat label="Active keys" value={6} />
      </div>
      <Panel>
        <PanelHead title="Usage — last 14 days" />
        <div style={{ padding: 18 }}>
          <UsageChart
            values={[120, 240, 180, 320, 290, 410, 380, 460, 520, 490, 610, 580, 700, 660]}
            label="Requests per day"
            height={150}
          />
        </div>
      </Panel>
      <Panel>
        <PanelHead title="Spend by model" />
        <div style={{ padding: 18 }}>
          <SpendBreakdown
            label="spend by model"
            formatValue={(v) => `$${v.toFixed(2)}`}
            rows={[
              { label: "claude-opus-4-8", value: 24.1 },
              { label: "claude-sonnet-5", value: 12.4 },
              { label: "claude-haiku-4-5", value: 4.7 },
            ]}
          />
        </div>
      </Panel>
      <Panel>
        <PanelHead title="Recent keys" />
        <Table>
          <thead>
            <tr>
              <th>Name</th>
              <th>Key</th>
              <th className="num">Budget</th>
              <th className="num">Used</th>
            </tr>
          </thead>
          <Tbody>
            <Tr>
              <td>production-agent</td>
              <td className="mono dim">agk_7f3a…9c2e</td>
              <td className="num">$500</td>
              <td className="num">$212</td>
            </Tr>
            <Tr>
              <td>staging-eval</td>
              <td className="mono dim">agk_1bd4…77f0</td>
              <td className="num">$50</td>
              <td className="num">$8</td>
            </Tr>
            <Tr>
              <td>local-dev</td>
              <td className="mono dim">agk_e992…0a41</td>
              <td className="num">∞</td>
              <td className="num">$1</td>
            </Tr>
          </Tbody>
        </Table>
      </Panel>
    </div>
  );
}

function ToastDemo() {
  const { success, error } = useToast();
  useEffect(() => {
    success("Key created — agk_7f3a…9c2e");
    const t = setTimeout(() => error("Budget exceeded for staging-eval"), 900);
    return () => clearTimeout(t);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
  return (
    <Button
      onClick={() => {
        success("Key created — agk_7f3a…9c2e");
        setTimeout(() => error("Budget exceeded for staging-eval"), 500);
      }}
    >
      Replay toasts
    </Button>
  );
}

function Feedback() {
  const [modalOpen, setModalOpen] = useState(true);
  return (
    <ToastProvider>
      <div style={{ display: "flex", flexDirection: "column", gap: 22 }}>
        <section>
          <div className="pv-label">Toasts — bottom-right, auto-dismiss</div>
          <ToastDemo />
        </section>
        <section>
          <div className="pv-label">Modal</div>
          <Button onClick={() => setModalOpen(true)}>Open modal</Button>
          <Modal
            open={modalOpen}
            onClose={() => setModalOpen(false)}
            title="Revoke key?"
            actions={
              <>
                <Button onClick={() => setModalOpen(false)}>Cancel</Button>
                <Button variant="danger" onClick={() => setModalOpen(false)}>
                  Revoke
                </Button>
              </>
            }
          >
            <p>This immediately invalidates agk_7f3a…9c2e. Agents using it will fail.</p>
          </Modal>
        </section>
        <section>
          <div className="pv-label">Skeleton loading</div>
          <Card>
            <Skeleton width="40%" height={11} />
            <div style={{ height: 8 }} />
            <Skeleton width="70%" height={22} />
            <div style={{ height: 10 }} />
            <Skeleton lines={2} />
          </Card>
        </section>
        <section>
          <div className="pv-label">Empty state</div>
          <Panel>
            <EmptyState
              title="No documents yet"
              description="Upload a document to give agents something to retrieve."
              action={<Button variant="primary">Upload document</Button>}
            />
          </Panel>
        </section>
      </div>
    </ToastProvider>
  );
}

function Shell() {
  const [path, setPath] = useState("/");
  return (
    <div style={{ display: "flex", height: "100vh", margin: -24 }}>
      <Sidebar
        routes={[
          { path: "/", label: "Overview" },
          { path: "/keys", label: "Keys" },
          { path: "/audit", label: "Audit" },
          { path: "/playground", label: "Playground" },
          { path: "/documents", label: "Documents" },
          { path: "/improve", label: "Improve" },
        ]}
        activePath={path}
        onNavigate={setPath}
        adminKey=""
        role="root"
        openSettings={() => {}}
      />
      <div style={{ flex: 1, padding: 32 }}>
        <h1 style={{ fontSize: 20, fontWeight: 600, letterSpacing: "-0.01em" }}>
          {path === "/" ? "Overview" : path.slice(1)[0].toUpperCase() + path.slice(2)}
        </h1>
        <p className="dim" style={{ marginTop: 4 }}>
          Click nav items — the active pill slides; the brand dot is quiet with no live runs.
        </p>
      </div>
    </div>
  );
}

const CARDS: Record<string, () => React.JSX.Element> = {
  core: Core,
  data: Data,
  feedback: Feedback,
  shell: Shell,
};

const card = document.body.dataset.card ?? "core";
const Showcase = CARDS[card] ?? Core;
createRoot(document.getElementById("root")!).render(
  <div className="pv-wrap">
    <Showcase />
  </div>,
);
