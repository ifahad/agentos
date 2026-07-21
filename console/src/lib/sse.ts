// Minimal SSE wire parser for POST-initiated streams (fetch + ReadableStream —
// EventSource cannot POST). Feed it decoded text chunks; it yields the payload
// of each complete event (the concatenated `data:` lines).

export class SSEParser {
  private buffer = "";
  private dataLines: string[] = [];

  /** Push a decoded chunk; returns the data payloads of any completed events. */
  push(chunk: string): string[] {
    this.buffer += chunk;
    const events: string[] = [];
    let idx: number;
    // Process every complete line; a blank line terminates an event.
    while ((idx = this.buffer.search(/\r?\n/)) !== -1) {
      const line = this.buffer.slice(0, idx);
      this.buffer = this.buffer.slice(idx + (this.buffer[idx] === "\r" ? 2 : 1));
      if (line === "") {
        if (this.dataLines.length > 0) {
          events.push(this.dataLines.join("\n"));
          this.dataLines = [];
        }
        continue;
      }
      if (line.startsWith("data:")) {
        // Spec: a single leading space after the colon is stripped.
        this.dataLines.push(line.slice(line.startsWith("data: ") ? 6 : 5));
      }
      // Comments (":...") and other fields (event:, id:, retry:) are ignored.
    }
    return events;
  }

  /** Flush a trailing event not terminated by a blank line (stream end). */
  flush(): string[] {
    const events = this.push("\n\n");
    this.buffer = "";
    return events;
  }
}

/**
 * POST `spec` and stream SSE events, invoking `onData` with each event's data
 * payload as it arrives. Resolves when the stream closes.
 */
export async function streamSSE(
  spec: { url: string; init: RequestInit },
  onData: (data: string) => void,
  signal?: AbortSignal,
): Promise<void> {
  const res = await fetch(spec.url, { ...spec.init, signal });
  if (!res.ok || !res.body) {
    let message = `HTTP ${res.status}`;
    try {
      const body = (await res.json()) as { detail?: unknown; error?: { message?: string } };
      if (typeof body?.detail === "string") message = body.detail;
      else if (body?.error?.message) message = body.error.message;
    } catch {
      // keep default message
    }
    throw new Error(message);
  }
  const reader = res.body.getReader();
  const decoder = new TextDecoder();
  const parser = new SSEParser();
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    for (const data of parser.push(decoder.decode(value, { stream: true }))) {
      onData(data);
    }
  }
  for (const data of parser.flush()) {
    onData(data);
  }
}
