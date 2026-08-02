import { useState, type DragEvent } from "react";
import type { PageProps } from "../App";
import { ErrorNotice, PageHead, errorMessage, useLoad } from "../components/common";
import { apiFetch, runtimeRequest } from "../lib/api";
import { formatInt } from "../lib/format";
import { documentsSummary } from "../lib/summaries";
import type { DocumentInfo } from "../lib/types";
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
  Textarea,
  Tr,
  useToast,
} from "../ui";
import "./Documents.css";

// Toasts come from the global <ToastProvider> mounted in App.
export function Documents(_props: PageProps) {
  const { data, error, loading, reload } = useLoad(
    () => apiFetch<DocumentInfo[]>(runtimeRequest("/documents")),
    [],
  );
  const toast = useToast();

  const [name, setName] = useState("");
  const [text, setText] = useState("");
  const [adding, setAdding] = useState(false);
  const [addError, setAddError] = useState<string | null>(null);
  const [dragging, setDragging] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);

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
      toast.success(
        `Ingested ${res.name} as ${formatInt(res.chunks)} chunk${res.chunks === 1 ? "" : "s"}.`,
      );
      setName("");
      setText("");
      setCreateOpen(false);
      reload();
    } catch (err) {
      const msg = errorMessage(err);
      setAddError(msg);
      toast.error(msg);
    } finally {
      setAdding(false);
    }
  };

  const onDragOver = (e: DragEvent) => {
    e.preventDefault();
    setDragging(true);
  };

  const onDragLeave = () => setDragging(false);

  const onDrop = (e: DragEvent) => {
    e.preventDefault();
    setDragging(false);
    const file = e.dataTransfer.files?.[0];
    if (!file) return;
    if (!file.type.startsWith("text/") && !/\.(txt|md|markdown)$/i.test(file.name)) {
      toast.error("Only plain-text files (.txt, .md) can be dropped here.");
      return;
    }
    setCreateOpen(true);
    void file.text().then((content) => {
      setText(content);
      setName((n) => n || file.name.replace(/\.[^.]+$/, ""));
    });
  };

  const docs = data ?? [];

  return (
    <>
      <PageHead
        title="Documents"
        subtitle="Knowledge base for the agent's search_knowledge tool — text is chunked, embedded through the gateway and stored in pgvector."
      />
      <ErrorNotice error={error} />

      <Panel
        className={`doc-drop${dragging ? " dragging" : ""}`}
        onDragOver={onDragOver}
        onDragLeave={onDragLeave}
        onDrop={onDrop}
      >
        <PanelHead
          title="Ingested documents"
          summary={documentsSummary(docs)}
          actions={
            <Button
              variant="primary"
              icon="plus"
              aria-expanded={createOpen}
              aria-controls="documents-create"
              onClick={() => setCreateOpen((v) => !v)}
            >
              Add document
            </Button>
          }
        />
        <Disclosure open={createOpen} onOpenChange={setCreateOpen} id="documents-create">
          <div className="panel-body">
            <ErrorNotice error={addError} />
            <Input
              label="Name"
              type="text"
              value={name}
              placeholder="q3-runbook"
              onChange={(e) => setName(e.target.value)}
            />
            <Textarea
              label="Text"
              rows={8}
              value={text}
              placeholder="Paste the document text to ingest…"
              onChange={(e) => setText(e.target.value)}
            />
            <p className="doc-hint">
              You can also drop a .txt or .md file anywhere on this panel to fill the form.
            </p>
            <Button
              variant="primary"
              icon="plus"
              onClick={() => void add()}
              disabled={adding || !name.trim() || !text.trim()}
            >
              {adding ? "Ingesting…" : "Ingest document"}
            </Button>
            {adding && <div className="doc-progress" aria-hidden />}
          </div>
        </Disclosure>
        <Table>
          <thead>
            <tr>
              <th>Name</th>
              <th className="num">Chunks</th>
            </tr>
          </thead>
          {loading && docs.length === 0 ? (
            <tbody>
              {Array.from({ length: 4 }, (_, i) => (
                <tr key={i}>
                  <td>
                    <Skeleton width="55%" />
                  </td>
                  <td className="num">
                    <Skeleton width={40} style={{ marginLeft: "auto" }} />
                  </td>
                </tr>
              ))}
            </tbody>
          ) : (
            <Tbody staggerKey={docs.length}>
              {docs.map((d) => (
                <Tr key={d.name}>
                  <td className="mono">{d.name}</td>
                  <td className="num">{formatInt(d.chunks)}</td>
                </Tr>
              ))}
              {docs.length === 0 && (
                <Tr animate={false}>
                  <td colSpan={2}>
                    <EmptyState
                      title="No documents ingested yet"
                      description="A document is chunked, embedded through the gateway and stored in pgvector — once ingested, the agent's search_knowledge tool can retrieve it."
                      action={
                        <Button variant="primary" icon="plus" onClick={() => setCreateOpen(true)}>
                          Add document
                        </Button>
                      }
                    />
                  </td>
                </Tr>
              )}
            </Tbody>
          )}
        </Table>
      </Panel>
    </>
  );
}
