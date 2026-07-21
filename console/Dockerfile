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
FROM nginx:alpine
COPY nginx.conf.template /etc/nginx/templates/default.conf.template
COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh
COPY --from=build /app/dist /usr/share/nginx/html
EXPOSE 80
ENTRYPOINT ["/docker-entrypoint.sh"]
