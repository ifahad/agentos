# The landing page has no build step: index.html is a single self-contained
# file with its fonts base64-inlined and no external requests, so there is
# nothing to compile and no node stage to run. Copy it and serve it.
#
# nginx-unprivileged runs the master process as uid 101 instead of root. It
# listens on 8080 rather than 80 because an unprivileged process cannot bind a
# port below 1024 — the published host port is unchanged, only the in-container
# one moves. Unlike the console this image needs no entrypoint: nothing here is
# templated, so the config is copied straight into conf.d and only ever read.
FROM nginxinc/nginx-unprivileged:alpine
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY index.html /usr/share/nginx/html/index.html
EXPOSE 8080
