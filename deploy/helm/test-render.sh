#!/usr/bin/env bash
# Render-level tests for the agentos Helm chart: helm lint + helm template
# under several value combinations, spot-asserting the rendered manifests.
set -euo pipefail

CHART_DIR="$(cd "$(dirname "$0")/agentos" && pwd)"
RELEASE=agentos # fullname collapses to "agentos" -> services agentos-<svc>

# The chart requires an explicit runtimeAuthToken (no default: a shipped one is
# a published credential). Every render below passes this, INCLUDING the
# negative tests — without it those would still fail, but for the missing token
# rather than the condition under test, and would report "ok" while proving
# nothing.
TOKEN=(--set runtimeAuthToken=render-test-token)

command -v helm >/dev/null || {
    echo "helm binary not found in PATH" >&2
    exit 1
}

fails=0
assert_contains() { # <name> <needle> ; input on stdin via $rendered
    if grep -qF -- "$2" <<<"$rendered"; then
        echo "  ok: $1"
    else
        echo "  FAIL: $1 (missing: $2)" >&2
        fails=$((fails + 1))
    fi
}
assert_not_contains() {
    if grep -qF -- "$2" <<<"$rendered"; then
        echo "  FAIL: $1 (unexpected: $2)" >&2
        fails=$((fails + 1))
    else
        echo "  ok: $1"
    fi
}

echo "== helm lint"
helm lint "$CHART_DIR" --strict

echo "== template: defaults"
rendered=$(helm template "$RELEASE" "$CHART_DIR" "${TOKEN[@]}")
assert_contains "gateway image"        'image: "agentos/gateway:0.3.0"'
assert_contains "runtime image"        'image: "agentos/runtime:0.3.0"'
assert_contains "sandbox image"        'image: "agentos/sandbox:0.3.0"'
assert_contains "console image"        'image: "agentos/console:0.3.0"'
assert_contains "landing image"        'image: "agentos/landing:0.3.0"'
assert_contains "postgres image"       'image: "pgvector/pgvector:pg16"'
assert_contains "postgres statefulset" 'kind: StatefulSet'
assert_contains "initdb seed in ConfigMap" 'CREATE DATABASE legacy_erp;'
assert_contains "runtime -> gateway URL"   'value: "http://agentos-gateway:8080"'
assert_contains "runtime -> sandbox URL"   'value: "http://agentos-sandbox:8070"'
assert_contains "MCP servers = sql only"   'value: "http://agentos-sql-connector:8090/mcp"'
assert_contains "gateway DB url via secret"    'key: database-url'
assert_contains "bootstrap keys composed"      'bootstrap-keys: "runtime:agos-local-dev-runtime:25"'
assert_contains "bundled DB url"               'database-url: "postgres://agentos:agentos@agentos-postgres:5432/agentos"'
assert_contains "connector DB url"             'connector-database-url: "postgres://erp_reader:erp_reader@agentos-postgres:5432/legacy_erp"'
assert_contains "sandbox read-only rootfs"     'readOnlyRootFilesystem: true'
assert_contains "sandbox no priv escalation"   'allowPrivilegeEscalation: false'
assert_contains "sandbox caps dropped"         '- ALL'
assert_contains "sandbox tmp emptyDir"         'mountPath: /tmp'
assert_contains "sandbox mem limit"            'memory: 768Mi'
assert_contains "console proxies gateway"      'proxy_pass http://agentos-gateway:8080/;'
assert_contains "console proxies runtime"      'proxy_pass http://agentos-runtime:8000/;'
assert_contains "sandbox NetworkPolicy rendered"    'kind: NetworkPolicy'
assert_not_contains "rest-connector off by default" 'agentos-rest-connector'
assert_not_contains "soap-connector off by default" 'agentos-soap-connector'
assert_not_contains "browser-connector off by default" 'agentos-browser-connector'
assert_not_contains "demo-crm off by default"       'agentos-demo-crm'
assert_not_contains "no ingress by default"         'kind: Ingress'
assert_not_contains "MCP list has no rest entry"    '8091/mcp'

echo "== template: external database (postgres.enabled=false)"
rendered=$(helm template "$RELEASE" "$CHART_DIR" "${TOKEN[@]}" \
    --set postgres.enabled=false \
    --set externalDatabaseUrl=postgres://u:p@db.example.com:5432/agentos \
    --set sqlConnector.databaseUrl=postgres://erp:erp@db.example.com:5432/legacy_erp)
assert_not_contains "no StatefulSet"        'kind: StatefulSet'
assert_not_contains "no pgvector image"     'pgvector/pgvector'
assert_contains "external DB url in secret" 'database-url: "postgres://u:p@db.example.com:5432/agentos"'
assert_contains "external connector DB url" 'connector-database-url: "postgres://erp:erp@db.example.com:5432/legacy_erp"'

echo "== template: rest-connector + demo-crm enabled"
rendered=$(helm template "$RELEASE" "$CHART_DIR" "${TOKEN[@]}" \
    --set restConnector.enabled=true \
    --set demoCrm.enabled=true \
    --set providerKeys.existingSecret=agentos-provider-keys)
assert_contains "rest-connector image" 'image: "agentos/rest-connector:0.3.0"'
assert_contains "demo-crm image"       'image: "agentos/demo-crm:0.3.0"'
assert_contains "MCP servers composed from both connectors" \
    'value: "http://agentos-sql-connector:8090/mcp,http://agentos-rest-connector:8091/mcp"'
assert_contains "spec URL defaults to demo CRM" 'value: "http://agentos-demo-crm:8095/openapi.json"'
assert_contains "mutations gated off"           'value: "false"'
assert_contains "provider secret referenced"    'name: agentos-provider-keys'
assert_contains "anthropic key env"             'key: AGENTOS_ANTHROPIC_API_KEY'
assert_contains "provider keys optional"        'optional: true'

echo "== template: publishing the console without auth in front must fail"
# Reaching the console is equivalent to holding the runtime token, so enabling
# the Ingress is an explicit assertion that something authenticates in front of
# it (docs/security/trust-boundaries.md).
if helm template "$RELEASE" "$CHART_DIR" "${TOKEN[@]}" \
    --set ingress.enabled=true \
    --set ingress.host=agentos.example.com >/dev/null 2>&1; then
    echo "  FAIL: Ingress rendered without ingress.frontedByAuth" >&2
    fails=$((fails + 1))
else
    echo "  ok: Ingress refuses to render until frontedByAuth is asserted"
fi

echo "== template: ingress enabled"
rendered=$(helm template "$RELEASE" "$CHART_DIR" "${TOKEN[@]}" \
    --set ingress.enabled=true \
    --set ingress.frontedByAuth=true \
    --set ingress.host=agentos.example.com \
    --set ingress.className=nginx \
    --set ingress.tls[0].secretName=agentos-tls \
    --set ingress.tls[0].hosts[0]=agentos.example.com)
assert_contains "ingress rendered"      'kind: Ingress'
assert_contains "ingress host"          'host: "agentos.example.com"'
assert_contains "ingress class"         'ingressClassName: nginx'
assert_contains "ingress tls secret"    'secretName: agentos-tls'
assert_contains "ingress -> console svc" 'name: agentos-console'

echo "== schema: values.schema.json rejects what used to be silently ignored"
# helm validates values against values.schema.json before rendering. Without it
# `--set gateway.rateLimitRpm=60` exited 0 and rendered byte-identical output:
# an operator reaching for the obvious camelCase name got a successful upgrade
# and a no-op. Each case below is one that really did pass silently.
reject() { # <label> <--set expr> <expected fragment>
    if out=$(helm template "$RELEASE" "$CHART_DIR" "${TOKEN[@]}" --set "$2" 2>&1); then
        echo "  FAIL: $1 was accepted" >&2
        fails=$((fails + 1))
    elif grep -qF -- "$3" <<<"$out"; then
        echo "  ok: $1"
    else
        echo "  FAIL: $1 rejected for the wrong reason (wanted: $3)" >&2
        fails=$((fails + 1))
    fi
}
reject "unknown key on a service" gateway.rateLimitRpm=60 "Additional property rateLimitRpm is not allowed"
reject "misspelled top-level key"  gatway.enabled=true     "Additional property gatway is not allowed"
reject "invalid guardrails mode"   gateway.guardrailsMode=maybe "must be one of the following"
reject "wrong type for a port"     console.service.port=eighty  "Expected: integer"
# The console entrypoint substitutes this into nginx config, where '$' starts a
# variable reference; catching it here beats catching it at container start.
reject "a '$' in the runtime token" 'runtimeAuthToken=has$dollar' "Does not match pattern"

# Free-form pass-throughs must stay free-form, or the schema breaks real use.
rendered=$(helm template "$RELEASE" "$CHART_DIR" "${TOKEN[@]}" \
    --set gateway.resources.limits.cpu=2 \
    --set 'gateway.extraEnv[0].name=CUSTOM' --set 'gateway.extraEnv[0].value=v')
assert_contains "resources still pass through" 'cpu: 2'
assert_contains "extraEnv still passes through" 'name: CUSTOM'

echo "== structural: every workload runs non-root with a uid, or is a named exception"
# Two failure modes, neither visible to a per-service string assertion:
#
#   1. `runAsNonRoot: true` with no `runAsUser` passes helm lint and then fails
#      at container-create time with CreateContainerConfigError, because the
#      kubelet cannot resolve an image's `USER <name>` to a numeric id.
#   2. No securityContext at all — silently root.
#
# The first version of this check grepped for runAsNonRoot FIRST and skipped
# anything lacking it, which meant the one workload in case 2 was precisely the
# one it ignored: it read as coverage while proving nothing about it. Enumerate
# every workload instead and make each exception explicit.
#
# postgres is the documented exception: the upstream pgvector entrypoint
# initialises its data directory as root on first start.
NONROOT_EXCEPTIONS="postgres.yaml"
ALL_ON=(--set restConnector.enabled=true
        --set restConnector.specUrl=http://spec.example.com/openapi.json
        --set demoCrm.enabled=true
        --set soapConnector.enabled=true
        --set soapConnector.wsdlUrl=http://legacy.example.com/svc?wsdl
        --set browserConnector.enabled=true)
# A render FAILURE and a disabled template both produce no output, so a bare
# `|| continue` would quietly check nothing and still print a clean run — the
# same silence-on-skip this check exists to catch. Count what was actually
# examined and fail below if the count collapses (cf. MIN_MEASURED in
# console/scripts/render.py).
checked=0
MIN_WORKLOADS=11
for tpl in "$CHART_DIR"/templates/*.yaml; do
    name=$(basename "$tpl")
    out=$(helm template "$RELEASE" "$CHART_DIR" "${TOKEN[@]}" "${ALL_ON[@]}" \
        --show-only "templates/$name" 2>/dev/null) || continue
    # Only workloads run containers; Services/Secrets/NetworkPolicies do not.
    grep -qE '^kind: (Deployment|StatefulSet|DaemonSet|Job|CronJob)' <<<"$out" || continue
    checked=$((checked + 1))
    if grep -qF " $name " <<<" $NONROOT_EXCEPTIONS "; then
        echo "  ok: $name is a documented root exception"
    elif ! grep -q 'runAsNonRoot: true' <<<"$out"; then
        echo "  FAIL: $name is a workload with no runAsNonRoot and no documented exception" >&2
        fails=$((fails + 1))
    elif grep -q 'runAsUser:' <<<"$out"; then
        echo "  ok: $name runs non-root and names a uid"
    else
        echo "  FAIL: $name sets runAsNonRoot: true with no runAsUser" >&2
        fails=$((fails + 1))
    fi
done
if [ "$checked" -lt "$MIN_WORKLOADS" ]; then
    echo "  FAIL: only $checked workload(s) examined, expected >= $MIN_WORKLOADS —" >&2
    echo "        a render error is being swallowed as 'disabled'" >&2
    fails=$((fails + 1))
else
    echo "  ok: $checked workloads examined (floor $MIN_WORKLOADS)"
fi

echo "== template: the runtime auth token reaches everything that needs it"
# The runtime calls require_runtime_auth_token() in lifespan() and refuses to
# start without this, so a chart that omits it does not degrade — it never boots.
rendered=$(helm template "$RELEASE" "$CHART_DIR" "${TOKEN[@]}")
assert_contains "secret carries the runtime auth token" 'runtime-auth-token:'
assert_contains "something consumes it"                 'key: runtime-auth-token'
rendered=$(helm template "$RELEASE" "$CHART_DIR" "${TOKEN[@]}" --show-only templates/runtime.yaml)
assert_contains "runtime is given the auth token" 'key: runtime-auth-token'

echo "== template: console nginx wiring"
rendered=$(helm template "$RELEASE" "$CHART_DIR" "${TOKEN[@]}" --show-only templates/console.yaml)
# The image entrypoint renders templates/default.conf.template INTO
# conf.d/default.conf under `set -eu`. Mounting the output path read-only makes
# that write fail and the container exit before nginx starts.
assert_contains "console mounts the template, not the rendered output" \
    'mountPath: /etc/nginx/templates/default.conf.template'
assert_not_contains "console must not mount over the entrypoint's output" \
    'mountPath: /etc/nginx/conf.d/default.conf'
assert_contains "console listens on the unprivileged port" 'listen 8080;'
assert_not_contains "console must not listen on a privileged port" 'listen 80;'
assert_contains "console injects the runtime bearer server-side" \
    'proxy_set_header Authorization "Bearer ${AGENTOS_RUNTIME_AUTH_TOKEN}"'
assert_contains "console is given the token to inject" 'key: runtime-auth-token'

echo "== template: soap + browser connectors enabled"
rendered=$(helm template "$RELEASE" "$CHART_DIR" "${TOKEN[@]}" \
    --set soapConnector.enabled=true \
    --set soapConnector.wsdlUrl=http://legacy.example.com/svc?wsdl \
    --set browserConnector.enabled=true \
    --set browserConnector.allowDomains=docs.example.com)
assert_contains "soap-connector image"    'image: "agentos/soap-connector:0.3.0"'
assert_contains "browser-connector image" 'image: "agentos/browser-connector:0.3.0"'
assert_contains "soap wsdl url"           'value: "http://legacy.example.com/svc?wsdl"'
assert_contains "browser allowlist"       'value: "docs.example.com"'
# Every MCP connector must reach the runtime's server list, or it is deployed
# and unreachable — the failure mode that is invisible until an agent needs it.
assert_contains "MCP servers include soap + browser" \
    'value: "http://agentos-sql-connector:8090/mcp,http://agentos-soap-connector:8093/mcp,http://agentos-browser-connector:8094/mcp"'
# tcpSocket, never httpGet: a GET on a streamable-HTTP MCP endpoint opens an
# event stream that never closes, so an HTTP probe hangs and kills a healthy pod.
assert_contains "connectors probe by tcpSocket" 'tcpSocket:'
assert_not_contains "no httpGet probe on connectors" 'path: /mcp'

echo "== template: soap-connector without a WSDL must fail"
if helm template "$RELEASE" "$CHART_DIR" "${TOKEN[@]}" --set soapConnector.enabled=true >/dev/null 2>&1; then
    echo "  FAIL: expected a 'soapConnector.wsdlUrl is required' error" >&2
    fails=$((fails + 1))
else
    echo "  ok: missing wsdlUrl rejected"
fi

echo "== template: landing"
rendered=$(helm template "$RELEASE" "$CHART_DIR" "${TOKEN[@]}" --show-only templates/landing.yaml)
assert_contains "landing deployment"        'kind: Deployment'
assert_contains "landing service"           'kind: Service'
assert_contains "landing runs nonroot"      'runAsNonRoot: true'
assert_contains "landing unprivileged port" 'containerPort: 8080'
# The landing page's whole claim is that it holds no credential and depends on
# nothing. Both are asserted rather than trusted to stay true.
assert_not_contains "landing mounts no secret"  'secretKeyRef'
assert_not_contains "landing has no ingress unless asked" 'kind: Ingress'

echo "== template: landing ingress enabled"
rendered=$(helm template "$RELEASE" "$CHART_DIR" "${TOKEN[@]}" \
    --set landing.ingress.enabled=true \
    --set landing.ingress.host=agentos.example.com \
    --set landing.ingress.className=nginx \
    --set landing.ingress.tls[0].secretName=landing-tls \
    --show-only templates/landing.yaml)
assert_contains "landing ingress rendered"       'kind: Ingress'
assert_contains "landing ingress host"           'host: "agentos.example.com"'
assert_contains "landing ingress class"          'ingressClassName: nginx'
assert_contains "landing ingress tls secret"     'secretName: landing-tls'
assert_contains "landing ingress -> landing svc" 'name: agentos-landing'

echo "== template: landing disabled"
if helm template "$RELEASE" "$CHART_DIR" "${TOKEN[@]}" \
    --set landing.enabled=false \
    --show-only templates/landing.yaml >/dev/null 2>&1; then
    echo "  FAIL: landing rendered despite landing.enabled=false" >&2
    fails=$((fails + 1))
else
    echo "  ok: no landing when landing.enabled=false"
fi

echo "== template: landing ingress without a host must fail"
if helm template "$RELEASE" "$CHART_DIR" "${TOKEN[@]}" \
    --set landing.ingress.enabled=true >/dev/null 2>&1; then
    echo "  FAIL: expected a 'requires landing.ingress.host' error" >&2
    fails=$((fails + 1))
else
    echo "  ok: landing ingress without a host rejected"
fi

echo "== template: sandbox NetworkPolicy (egress-less topology)"
rendered=$(helm template "$RELEASE" "$CHART_DIR" "${TOKEN[@]}" \
    --show-only templates/sandbox-networkpolicy.yaml)
assert_contains "netpol selects sandbox pod"      'app.kubernetes.io/component: sandbox'
assert_contains "netpol governs Ingress"          '- Ingress'
assert_contains "netpol governs Egress"           '- Egress'
assert_contains "netpol ingress from runtime pod" 'app.kubernetes.io/component: runtime'
assert_contains "netpol ingress on sandbox port"  'port: 8070'
assert_contains "netpol egress only to kube-dns"  'k8s-app: kube-dns'
assert_contains "netpol kube-system namespace"    'kubernetes.io/metadata.name: kube-system'
assert_contains "netpol DNS port"                 'port: 53'
assert_contains "netpol DNS over UDP"             'protocol: UDP'

echo "== template: sandbox NetworkPolicy disabled"
if helm template "$RELEASE" "$CHART_DIR" "${TOKEN[@]}" \
    --set sandbox.networkPolicy.enabled=false \
    --show-only templates/sandbox-networkpolicy.yaml >/dev/null 2>&1; then
    echo "  FAIL: NetworkPolicy rendered despite networkPolicy.enabled=false" >&2
    fails=$((fails + 1))
else
    echo "  ok: no NetworkPolicy when networkPolicy.enabled=false"
fi
if helm template "$RELEASE" "$CHART_DIR" "${TOKEN[@]}" \
    --set sandbox.enabled=false \
    --show-only templates/sandbox-networkpolicy.yaml >/dev/null 2>&1; then
    echo "  FAIL: NetworkPolicy rendered despite sandbox.enabled=false" >&2
    fails=$((fails + 1))
else
    echo "  ok: no NetworkPolicy when sandbox.enabled=false"
fi

echo "== template: rest-connector without spec must fail"
if helm template "$RELEASE" "$CHART_DIR" "${TOKEN[@]}" --set restConnector.enabled=true >/dev/null 2>&1; then
    echo "  FAIL: expected 'required' error for missing restConnector.specUrl" >&2
    fails=$((fails + 1))
else
    echo "  ok: missing specUrl rejected"
fi

echo "== template: postgres disabled without externalDatabaseUrl must fail"
if helm template "$RELEASE" "$CHART_DIR" "${TOKEN[@]}" --set postgres.enabled=false >/dev/null 2>&1; then
    echo "  FAIL: expected 'required' error for missing externalDatabaseUrl" >&2
    fails=$((fails + 1))
else
    echo "  ok: missing externalDatabaseUrl rejected"
fi

if [ "$fails" -ne 0 ]; then
    echo "$fails assertion(s) failed" >&2
    exit 1
fi
echo "all render checks passed"
