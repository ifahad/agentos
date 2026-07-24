import { useEffect, useRef, useState } from "react";
import { AnimatePresence, motion, useReducedMotion } from "framer-motion";
import type { PageProps } from "../App";
import { ErrorNotice, PageHead, errorMessage } from "../components/common";
import { apiFetchRaw, runtimeRequest } from "../lib/api";
import { compactJSON } from "../lib/format";
import { streamSSE } from "../lib/sse";
import type { PendingApprovalResponse, PendingTool, RunResponse, StreamEvent } from "../lib/types";
import {
  Button,
  EmptyState,
  Textarea,
  STAGGER,
  STAGGER_MAX_ITEMS,
  transition,
  transitionFast,
} from "../ui";
import "./Playground.css";

type Entry =
  | { kind: "user"; text: string }
  | { kind: "step"; tool: string; input: unknown }
  | { kind: "output"; text: string }
  | { kind: "pending"; pending: PendingTool[]; resolved: "approved" | "denied" | null }
  | { kind: "info"; text: string };

/** Pretty-printed tool input for the expanded tool-call body. */
function prettyJSON(value: unknown): string {
  try {
    return JSON.stringify(value, null, 2) ?? String(value);
  } catch {
    return String(value);
  }
}

/**
 * Expandable tool call — collapsed shows the tool name and a one-line summary;
 * expanded reveals the full input. Height animates via AnimatePresence.
 */
function ToolCall({ tool, input }: { tool: string; input: unknown }) {
  const [open, setOpen] = useState(false);
  const reduced = useReducedMotion();
  return (
    <div className="pg-tool">
      <button
        type="button"
        className="pg-tool-toggle"
        aria-expanded={open}
        onClick={() => setOpen((o) => !o)}
      >
        <span className={`pg-chevron${open ? " open" : ""}`} aria-hidden>
          ›
        </span>
        <span className="tool-name mono">{tool}</span>
        {!open && <code className="pg-tool-summary">{compactJSON(input, 90)}</code>}
      </button>
      <AnimatePresence initial={false}>
        {open && (
          <motion.div
            key="body"
            className="pg-tool-body"
            initial={reduced ? false : { height: 0, opacity: 0 }}
            animate={{ height: "auto", opacity: 1 }}
            exit={reduced ? { opacity: 0 } : { height: 0, opacity: 0 }}
            transition={transition}
          >
            <pre className="pg-tool-pre">{prettyJSON(input)}</pre>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}

export function Playground(_props: PageProps) {
  const [input, setInput] = useState("");
  const [streaming, setStreaming] = useState(false);
  const [entries, setEntries] = useState<Entry[]>([]);
  const [threadId, setThreadId] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const bottomRef = useRef<HTMLDivElement>(null);
  const reduced = useReducedMotion();

  // Length of `entries` at the previous commit — used to stagger only the
  // freshly appended batch (capped), so earlier events never re-animate.
  const prevCountRef = useRef(0);
  useEffect(() => {
    prevCountRef.current = entries.length;
  }, [entries.length]);

  const enterDelay = (i: number): number => {
    if (reduced) return 0;
    const fresh = i - prevCountRef.current;
    if (fresh < 0) return 0;
    return Math.min(fresh, STAGGER_MAX_ITEMS - 1) * STAGGER;
  };

  const eventMotion = (i: number) => ({
    initial: reduced ? false : { opacity: 0, y: 6 },
    animate: { opacity: 1, y: 0 },
    exit: reduced ? { opacity: 0 } : { opacity: 0, y: 4, transition: transitionFast },
    transition: { ...transition, delay: enterDelay(i) },
  });

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
            <Button small onClick={reset}>
              New thread
            </Button>
          </>
        ) : (
          <>new conversation</>
        )}
      </div>

      <ErrorNotice error={error} />

      <div className="timeline pg-stream">
        {entries.length === 0 && !busy ? (
          <EmptyState
            title="No run yet"
            description="Ask the governed agent below — tool calls and answers stream in here as a live step stream."
          />
        ) : (
          <AnimatePresence initial={false}>
            {entries.map((e, i) => {
              switch (e.kind) {
                case "user":
                  return (
                    <motion.div key={i} className="event user" {...eventMotion(i)}>
                      <div className="event-tag">you</div>
                      <pre>{e.text}</pre>
                    </motion.div>
                  );
                case "step": {
                  const stepNo = entries
                    .slice(0, i + 1)
                    .reduce((n, x) => n + (x.kind === "step" ? 1 : 0), 0);
                  return (
                    <motion.div key={i} className="event step" {...eventMotion(i)}>
                      <div className="event-tag">step {stepNo} · tool call</div>
                      <ToolCall tool={e.tool} input={e.input} />
                    </motion.div>
                  );
                }
                case "output":
                  return (
                    <motion.div key={i} className="event output" {...eventMotion(i)}>
                      <div className="event-tag">agent</div>
                      <pre>{e.text}</pre>
                    </motion.div>
                  );
                case "pending":
                  return (
                    <motion.div key={i} className="event pending" {...eventMotion(i)}>
                      <div className="event-tag">
                        {e.resolved === null && (
                          <span className="pg-live-dot pg-live-dot--amber" aria-hidden />
                        )}
                        approval required{e.resolved ? ` — ${e.resolved}` : ""}
                      </div>
                      {e.pending.map((p, j) => (
                        <ToolCall key={j} tool={p.tool} input={p.input} />
                      ))}
                      {e.resolved === null && (
                        <div className="pending-actions">
                          <Button
                            small
                            variant="primary"
                            disabled={busy}
                            onClick={() => void decide(i, true)}
                          >
                            Approve
                          </Button>
                          <Button
                            small
                            variant="danger"
                            disabled={busy}
                            onClick={() => void decide(i, false)}
                          >
                            Deny
                          </Button>
                        </div>
                      )}
                    </motion.div>
                  );
                case "info":
                  return (
                    <motion.div key={i} className="event" {...eventMotion(i)}>
                      <pre className="muted">{e.text}</pre>
                    </motion.div>
                  );
              }
            })}
            {busy && (
              <motion.div
                key="thinking"
                className="event pg-thinking"
                initial={reduced ? false : { opacity: 0 }}
                animate={{ opacity: 1 }}
                exit={{ opacity: 0 }}
                transition={transition}
              >
                <div className="event-tag">
                  <span className="pg-live-dot" aria-hidden /> working
                </div>
                <div className="pg-shimmer w70" />
                <div className="pg-shimmer w45" />
              </motion.div>
            )}
          </AnimatePresence>
        )}
        <div ref={bottomRef} />
      </div>

      <div className="run-bar">
        <Textarea
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
          <Button
            variant="primary"
            onClick={() => void run()}
            disabled={busy || !input.trim() || hasUnresolvedPending}
          >
            {busy ? "Running…" : "Run"}
          </Button>
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
