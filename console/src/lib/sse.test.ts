import { describe, expect, it } from "vitest";
import { SSEParser } from "./sse";

describe("SSEParser", () => {
  it("parses a single complete event", () => {
    const p = new SSEParser();
    expect(p.push('data: {"event":"done"}\n\n')).toEqual(['{"event":"done"}']);
  });

  it("parses multiple events in one chunk", () => {
    const p = new SSEParser();
    expect(p.push("data: one\n\ndata: two\n\n")).toEqual(["one", "two"]);
  });

  it("handles events split across arbitrary chunk boundaries", () => {
    const p = new SSEParser();
    expect(p.push('data: {"ev')).toEqual([]);
    expect(p.push('ent":"step"')).toEqual([]);
    expect(p.push("}\n")).toEqual([]);
    expect(p.push("\n")).toEqual(['{"event":"step"}']);
  });

  it("handles CRLF line endings", () => {
    const p = new SSEParser();
    expect(p.push("data: hello\r\n\r\n")).toEqual(["hello"]);
  });

  it("joins multiple data lines of one event with newlines", () => {
    const p = new SSEParser();
    expect(p.push("data: line1\ndata: line2\n\n")).toEqual(["line1\nline2"]);
  });

  it("ignores comments and non-data fields", () => {
    const p = new SSEParser();
    expect(p.push(": keepalive\nevent: step\nid: 7\ndata: payload\n\n")).toEqual(["payload"]);
  });

  it("does not strip more than one space after the colon", () => {
    const p = new SSEParser();
    expect(p.push("data:  spaced\n\n")).toEqual([" spaced"]);
  });

  it("handles a no-space data field", () => {
    const p = new SSEParser();
    expect(p.push("data:tight\n\n")).toEqual(["tight"]);
  });

  it("emits nothing for blank keepalive lines", () => {
    const p = new SSEParser();
    expect(p.push("\n\n\n")).toEqual([]);
  });

  it("flushes a trailing unterminated event at stream end", () => {
    const p = new SSEParser();
    expect(p.push("data: tail")).toEqual([]);
    expect(p.flush()).toEqual(["tail"]);
    expect(p.flush()).toEqual([]);
  });

  it("passes [DONE] sentinels through untouched", () => {
    const p = new SSEParser();
    expect(p.push("data: [DONE]\n\n")).toEqual(["[DONE]"]);
  });
});
