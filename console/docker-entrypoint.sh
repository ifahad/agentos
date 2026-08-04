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

# A '$' in the token gets substituted into the config verbatim and nginx then
# parses it as a variable reference — `[emerg] unknown "foo" variable`, exit
# before it ever binds. nginx has no escape for a literal '$' inside a quoted
# string, so the token genuinely cannot contain one. Fail here, naming the
# cause, rather than leaving an operator to decode nginx's version of it.
case $AGENTOS_RUNTIME_AUTH_TOKEN in
*'$'*)
    echo "agentos-console: AGENTOS_RUNTIME_AUTH_TOKEN must not contain '\$'." >&2
    echo "  nginx would read it as a variable reference and refuse to start." >&2
    echo "  Hex and base64 tokens are safe; regenerate without a '\$'." >&2
    exit 1
    ;;
esac

envsubst '${AGENTOS_RUNTIME_AUTH_TOKEN}' \
    < /etc/nginx/templates/default.conf.template \
    > /etc/nginx/conf.d/default.conf

exec nginx -g 'daemon off;'
