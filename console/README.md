# console/ — the AgentOS operator surface

TypeScript admin console for AgentOS. Vite + React + TypeScript SPA,
hand-rolled CSS (no component library), served by nginx in production.

For what each page does and who can see it, see
[`../docs/console.md`](../docs/console.md).

## Developing

```bash
cd console
npm install
npm run dev      # Vite dev server
npm test -- --run
npm run build
```

`make test-console` runs the vitest suite and the production build.

## Architecture notes

- Routing is a single `ROUTES` array in `src/App.tsx` driving both the sidebar
  and the ⌘K command palette — add a route there and both pick it up.
- The browser talks only to same-origin `/api/gateway/*` and `/api/runtime/*`;
  nginx injects the runtime auth token server-side.
- Design system: graphite surfaces, colour reserved for machine state
  (in-flight / ok / awaiting-human / denied). Motion presets live in
  `src/ui/motion.ts` and reduced motion is honoured.
