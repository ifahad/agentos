# Build stage: compile the SPA.
FROM node:22-alpine AS build
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY tsconfig.json vite.config.ts index.html ./
COPY src ./src
RUN npm run build

# Serve stage: nginx with SPA fallback + API reverse proxy (no CORS needed).
# The runtime now requires a bearer token; a small entrypoint renders the
# config from its template at start-up, injecting AGENTOS_RUNTIME_AUTH_TOKEN via
# envsubst (shipped in nginx:alpine as part of gettext). No secret is baked in.
# nginx-unprivileged runs the master process as uid 101 instead of root. It
# listens on 8080 rather than 80 because an unprivileged process cannot bind a
# port below 1024 — the published host port is unchanged, only the in-container
# one moves.
FROM nginxinc/nginx-unprivileged:alpine
USER root
COPY nginx.conf.template /etc/nginx/templates/default.conf.template
COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh \
    && chown -R 101:101 /etc/nginx/conf.d
COPY --from=build /app/dist /usr/share/nginx/html
USER 101:101
EXPOSE 8080
ENTRYPOINT ["/docker-entrypoint.sh"]
