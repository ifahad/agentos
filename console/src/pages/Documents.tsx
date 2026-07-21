import { useState } from "react";
import type { PageProps } from "../App";
import { ErrorNotice, PageHead, errorMessage, useLoad } from "../components/common";
import { apiFetch, runtimeRequest } from "../lib/api";
import { formatInt } from "../lib/format";
import type { DocumentInfo } from "../lib/types";

export function Documents(_props: PageProps) {
  const { data, error, loading, reload } = useLoad(
    () => apiFetch<DocumentInfo[]>(runtimeRequest("/documents")),
    [],
  );

  const [name, setName] = useState("");
  const [text, setText] = useState("");
  const [adding, setAdding] = useState(false);
  const [addError, setAddError] = useState<string | null>(null);
  const [lastAdded, setLastAdded] = useState<DocumentInfo | null>(null);

  const add = async () => {
    if (!name.trim() || !text.trim()) {
      setAddError("Both a name and document text are required.");
      return;
    }
    setAdding(true);
    setAddError(null);
    try {
      const res = await apiFetch<DocumentInfo>(
        runtimeRequest("/documents", { name: name.trim(), text }),
      );
      setLastAdded(res);
      setName("");
      setText("");
      reload();
    } catch (err) {
      setAddError(errorMessage(err));
    } finally {
      setAdding(false);
    }
  };

  const docs = data ?? [];

  return (
    <>
      <PageHead
        title="Documents"
        subtitle="Knowledge base for the agent's search_knowledge tool — text is chunked, embedded through the gateway and stored in pgvector."
      />
      <ErrorNotice error={error} />

      {lastAdded && (
        <div className="notice">
          Ingested <span className="mono">{lastAdded.name}</span> as{" "}
          {formatInt(lastAdded.chunks)} chunk{lastAdded.chunks === 1 ? "" : "s"}.
        </div>
      )}

      <div className="panel">
        <div className="panel-head">
          <h2>Add document</h2>
        </div>
        <div className="panel-body">
          <ErrorNotice error={addError} />
          <label className="field">
            <span>Name</span>
            <input
              type="text"
              value={name}
              placeholder="q3-runbook"
              onChange={(e) => setName(e.target.value)}
            />
          </label>
          <label className="field">
            <span>Text</span>
            <textarea
              rows={8}
              value={text}
              placeholder="Paste the document text to ingest…"
              onChange={(e) => setText(e.target.value)}
            />
          </label>
          <button className="btn primary" onClick={() => void add()} disabled={adding}>
            {adding ? "Ingesting…" : "Ingest document"}
          </button>
        </div>
      </div>

      <div className="panel">
        <div className="panel-head">
          <h2>Ingested documents</h2>
          {loading && <span className="spin">loading…</span>}
        </div>
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Name</th>
                <th className="num">Chunks</th>
              </tr>
            </thead>
            <tbody>
              {docs.map((d) => (
                <tr key={d.name}>
                  <td className="mono">{d.name}</td>
                  <td className="num">{formatInt(d.chunks)}</td>
                </tr>
              ))}
              {docs.length === 0 && !loading && (
                <tr>
                  <td colSpan={2} className="empty">
                    No documents ingested yet.
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </div>
    </>
  );
}
