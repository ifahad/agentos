# AgentOS landing page

A single self-contained marketing page for AgentOS, built in the console's
"Instrument" design language (graphite surfaces, signal-only colour, engraved
mono legends, the governance-chain motif). Fonts (Archivo + IBM Plex Mono) are
embedded as data URIs, so it makes no external requests and works offline.

Open `index.html` directly in a browser, or serve the directory:

    python3 -m http.server --directory landing 8099

It is theme-aware (dark primary, light supported, plus a toggle) and responsive.
Content maps to real platform capabilities — governance chain, the capability
grid, Multiverse, Operators, and the open-source stack it is built on.
