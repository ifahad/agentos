import { useRef, useState } from "react";
import type { PageProps } from "../App";
import { ErrorNotice, PageHead, errorMessage } from "../components/common";
import { apiFetchRaw, runtimeRequest } from "../lib/api";
import { compactJSON } from "../lib/format";
import { streamSSE } from "../lib/sse";
import type { PendingApprovalResponse, PendingTool, RunResponse, StreamEvent } from "../lib/types";

type Entry =
  | { kind: "user"; text: string }
  | { kind: "step"; tool: string; input: unknown }
  | { kind: "output"; text: string }
  | { kind: "pending"; pending: PendingTool[]; resolved: "approved" | "denied" | null }
  | { kind: "info"; text: string };

export function Playground(_props: PageProps) {
  const [input, setInput] = useState("");
  const [streaming, setStreaming] = useState(false);
  const [entries, setEntries] = useState<Entry[]>([]);
  const [threadId, setThreadId] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const bottomRef = useRef<HTMLDivElement>(null);

  const append = (...added: Entry[]) => {
    setEntries((prev) => [...prev, ...added]);
    requestAnimationFrame(() => bottomRef.current?.scrollIntoView({ behavior: "smooth" }));
  };

  const hasUnresolvedPending = entries.some((e) => e.kind === "pending" && e.resolved === null);

  /** Fold a completed-run payload (from /runs, /approve or a done SSE event) into the timeline. */
  const renderCompleted = (res: RunResponse, includeSteps: boolean) => {
    setThreadId(res.thread_id);
    const added: Entry[] = [];
    if (includeSteps) {
      for (const s of res.steps) added.push({ kind: "step", tool: s.tool, input: s.input });
    }
    added.push({ kind: "output", text: res.output });
    append(...added);
  };

  const handleRunResult = (status: number, body: unknown, includeSteps: boolean) => {
    if (status === 202) {
      const pending = body as PendingApprovalResponse;
      setThreadId(pending.thread_id);
      append({ kind: "pending", pending: pending.pending, resolved: null });
    } else {
      renderCompleted(body as RunResponse, includeSteps);
    }
  };

  const run = async () => {
    const text = input.trim();
    if (!text || busy) return;
    setBusy(true);
    setError(null);
    setInput("");
    append({ kind: "user", text });

    // The streamed pending_approval event does not carry a thread id, so for
    // streaming runs we pick the thread id client-side up front — approvals
    // then always have an address.
    let tid = threadId;
    if (streaming && !tid) {
      tid = crypto.randomUUID().replaceAll("-", "");
      setThreadId(tid);
    }
    const body: Record<string, unknown> = { input: text };
    if (tid) body["thread_id"] = tid;

    try {
      if (streaming) {
        await streamSSE(runtimeRequest("/runs/stream", body), (data) => {
          if (data === "[DONE]") return;
          let ev: StreamEvent;
          try {
            ev = JSON.parse(data) as StreamEvent;
          } catch {
            return;
          }
          if (ev.event === "step") {
            append({ kind: "step", tool: ev.tool, input: ev.input });
          } else if (ev.event === "pending_approval") {
            if (ev.thread_id) setThreadId(ev.thread_id);
            append({ kind: "pending", pending: ev.pending, resolved: null });
          } else if (ev.event === "done") {
            // Steps already streamed live; only render the final output.
            renderCompleted(ev, false);
          }
        });
      } else {
        const { status, body: resBody } = await apiFetchRaw(runtimeRequest("/runs", body));
        handleRunResult(status, resBody, true);
      }
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  };

  const decide = async (index: number, approve: boolean) => {
    if (!threadId || busy) return;
    setBusy(true);
    setError(null);
    setEntries((prev) =>
      prev.map((e, i) =>
        i === index && e.kind === "pending"
          ? { ...e, resolved: approve ? "approved" : "denied" }
          : e,
      ),
    );
    try {
      const { status, body } = await apiFetchRaw(
        runtimeRequest(`/runs/${threadId}/approve`, { approve }),
      );
      handleRunResult(status, body, true);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  };

  const reset = () => {
    setEntries([]);
    setThreadId(null);
    setError(null);
  };

  return (
    <>
      <PageHead
        title="Playground"
        subtitle="Run the governed agent. Tool calls appear as a timeline; tools gated by human approval pause the run until you decide."
      />

      <div className="thread-line">
        {threadId ? (
          <>
            thread <span className="mono">{threadId}</span>
            <button className="btn small" onClick={reset}>
              New thread
            </button>
          </>
        ) : (
          <>new conversation</>
        )}
      </div>

      <ErrorNotice error={error} />

      <div className="timeline">
        {entries.map((e, i) => {
          switch (e.kind) {
            case "user":
              return (
                <div key={i} className="event user">
                  <div className="event-tag">you</div>
                  <pre>{e.text}</pre>
                </div>
              );
            case "step":
              return (
                <div key={i} className="event step">
                  <div className="event-tag">tool call</div>
                  <div>
                    <span className="tool-name mono">{e.tool}</span>{" "}
                    <code>{compactJSON(e.input)}</code>
                  </div>
                </div>
              );
            case "output":
              return (
                <div key={i} className="event output">
                  <div className="event-tag">agent</div>
                  <pre>{e.text}</pre>
                </div>
              );
            case "pending":
              return (
                <div key={i} className="event pending">
                  <div className="event-tag">
                    approval required{e.resolved ? ` — ${e.resolved}` : ""}
                  </div>
                  {e.pending.map((p, j) => (
                    <div key={j}>
                      <span className="tool-name mono">{p.tool}</span>{" "}
                      <code>{compactJSON(p.input)}</code>
                    </div>
                  ))}
                  {e.resolved === null && (
                    <div className="pending-actions">
                      <button
                        className="btn small primary"
                        disabled={busy}
                        onClick={() => void decide(i, true)}
                      >
                        Approve
                      </button>
                      <button
                        className="btn small danger"
                        disabled={busy}
                        onClick={() => void decide(i, false)}
                      >
                        Deny
                      </button>
                    </div>
                  )}
                </div>
              );
            case "info":
              return (
                <div key={i} className="event">
                  <pre className="muted">{e.text}</pre>
                </div>
              );
          }
        })}
        {busy && <div className="spin">running…</div>}
        <div ref={bottomRef} />
      </div>

      <div className="run-bar">
        <textarea
          value={input}
          placeholder="Ask the agent, e.g. “Which customer has the highest total order value?”"
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
              e.preventDefault();
              void run();
            }
          }}
        />
        <div className="run-controls">
          <button
            className="btn primary"
            onClick={() => void run()}
            disabled={busy || !input.trim() || hasUnresolvedPending}
          >
            {busy ? "Running…" : "Run"}
          </button>
          <label className="toggle">
            <input
              type="checkbox"
              checked={streaming}
              onChange={(e) => setStreaming(e.target.checked)}
            />
            Stream events
          </label>
        </div>
      </div>
    </>
  );
}
