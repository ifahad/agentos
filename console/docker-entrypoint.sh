#!/bin/sh
# Render the nginx config from its template at container start, injecting the
# runtime auth token so the browser never holds it.
#
# Only ${AGENTOS_RUNTIME_AUTH_TOKEN} is passed to envsubst (explicit var list),
# so nginx's own runtime variables ($host, $uri, ...) in the template are left
# untouched. If the var is unset an empty Bearer is injected and the runtime
# will 401 — compose/helm is expected to set a real token.
set -eu

: "${AGENTOS_RUNTIME_AUTH_TOKEN:=}"
export AGENTOS_RUNTIME_AUTH_TOKEN

envsubst '${AGENTOS_RUNTIME_AUTH_TOKEN}' \
    < /etc/nginx/templates/default.conf.template \
    > /etc/nginx/conf.d/default.conf

exec nginx -g 'daemon off;'
