import { afterEach } from "vitest";

// @testing-library/react unmounts between tests only when it can see a global
// afterEach, which it cannot here: this project keeps vitest `globals` off, so
// every existing suite imports what it uses. Without this, renders accumulate
// in one document and a query matches elements left behind by an earlier
// test — which reads as a bug in the component under test rather than in the
// harness. It cost two failures on the very first component suite.
//
// Guarded on `document` because this file is a setupFile for EVERY suite, and
// the 36 that predate the DOM harness run in the node environment where
// @testing-library/react cannot even be imported.
if (typeof document !== "undefined") {
  const { cleanup } = await import("@testing-library/react");
  afterEach(cleanup);
}
