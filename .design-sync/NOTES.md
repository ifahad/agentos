# Design sync notes — AgentOS Console

- Project: "AgentOS Console" (53f934b5-0289-4113-b170-ab81887c37e5), hand-managed (no converter run).
- Real component previews: source at console/design/preview/main.tsx (+ vite.config.ts).
  Rebuild after component changes: `cd console && npx vite build --config design/preview/vite.config.ts`
  → emits design/assets/agentos-console.{css,iife.js}; the @dsCard HTML shells reference those.
- Cards: design/components/ui-{core,data,feedback}.html, design/shell/sidebar.html render the REAL
  src/ui, src/charts, src/components/Sidebar components. design/tokens/*.html and
  design/playground/step-stream.html remain hand-authored pattern references.
- Sidebar preview passes adminKey="" so its live-dot polling is disabled inside iframes.
