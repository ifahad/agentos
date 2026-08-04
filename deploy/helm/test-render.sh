#!/usr/bin/env bash
# Render-level tests for the agentos Helm chart: helm lint + helm template
# under several value combinations, spot-asserting the rendered manifests.
set -euo pipefail

CHART_DIR="$(cd "$(dirname "$0")/agentos" && pwd)"
RELEASE=agentos # fullname collapses to "agentos" -> services agentos-<svc>

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
rendered=$(helm template "$RELEASE" "$CHART_DIR")
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
rendered=$(helm template "$RELEASE" "$CHART_DIR" \
    --set postgres.enabled=false \
    --set externalDatabaseUrl=postgres://u:p@db.example.com:5432/agentos \
    --set sqlConnector.databaseUrl=postgres://erp:erp@db.example.com:5432/legacy_erp)
assert_not_contains "no StatefulSet"        'kind: StatefulSet'
assert_not_contains "no pgvector image"     'pgvector/pgvector'
assert_contains "external DB url in secret" 'database-url: "postgres://u:p@db.example.com:5432/agentos"'
assert_contains "external connector DB url" 'connector-database-url: "postgres://erp:erp@db.example.com:5432/legacy_erp"'

echo "== template: rest-connector + demo-crm enabled"
rendered=$(helm template "$RELEASE" "$CHART_DIR" \
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

echo "== template: ingress enabled"
rendered=$(helm template "$RELEASE" "$CHART_DIR" \
    --set ingress.enabled=true \
    --set ingress.host=agentos.example.com \
    --set ingress.className=nginx \
    --set ingress.tls[0].secretName=agentos-tls \
    --set ingress.tls[0].hosts[0]=agentos.example.com)
assert_contains "ingress rendered"      'kind: Ingress'
assert_contains "ingress host"          'host: "agentos.example.com"'
assert_contains "ingress class"         'ingressClassName: nginx'
assert_contains "ingress tls secret"    'secretName: agentos-tls'
assert_contains "ingress -> console svc" 'name: agentos-console'

echo "== template: soap + browser connectors enabled"
rendered=$(helm template "$RELEASE" "$CHART_DIR" \
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
if helm template "$RELEASE" "$CHART_DIR" --set soapConnector.enabled=true >/dev/null 2>&1; then
    echo "  FAIL: expected a 'soapConnector.wsdlUrl is required' error" >&2
    fails=$((fails + 1))
else
    echo "  ok: missing wsdlUrl rejected"
fi

echo "== template: landing"
rendered=$(helm template "$RELEASE" "$CHART_DIR" --show-only templates/landing.yaml)
assert_contains "landing deployment"        'kind: Deployment'
assert_contains "landing service"           'kind: Service'
assert_contains "landing runs nonroot"      'runAsNonRoot: true'
assert_contains "landing unprivileged port" 'containerPort: 8080'
# The landing page's whole claim is that it holds no credential and depends on
# nothing. Both are asserted rather than trusted to stay true.
assert_not_contains "landing mounts no secret"  'secretKeyRef'
assert_not_contains "landing has no ingress unless asked" 'kind: Ingress'

echo "== template: landing ingress enabled"
rendered=$(helm template "$RELEASE" "$CHART_DIR" \
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
if helm template "$RELEASE" "$CHART_DIR" \
    --set landing.enabled=false \
    --show-only templates/landing.yaml >/dev/null 2>&1; then
    echo "  FAIL: landing rendered despite landing.enabled=false" >&2
    fails=$((fails + 1))
else
    echo "  ok: no landing when landing.enabled=false"
fi

echo "== template: landing ingress without a host must fail"
if helm template "$RELEASE" "$CHART_DIR" \
    --set landing.ingress.enabled=true >/dev/null 2>&1; then
    echo "  FAIL: expected a 'requires landing.ingress.host' error" >&2
    fails=$((fails + 1))
else
    echo "  ok: landing ingress without a host rejected"
fi

echo "== template: sandbox NetworkPolicy (egress-less topology)"
rendered=$(helm template "$RELEASE" "$CHART_DIR" \
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
if helm template "$RELEASE" "$CHART_DIR" \
    --set sandbox.networkPolicy.enabled=false \
    --show-only templates/sandbox-networkpolicy.yaml >/dev/null 2>&1; then
    echo "  FAIL: NetworkPolicy rendered despite networkPolicy.enabled=false" >&2
    fails=$((fails + 1))
else
    echo "  ok: no NetworkPolicy when networkPolicy.enabled=false"
fi
if helm template "$RELEASE" "$CHART_DIR" \
    --set sandbox.enabled=false \
    --show-only templates/sandbox-networkpolicy.yaml >/dev/null 2>&1; then
    echo "  FAIL: NetworkPolicy rendered despite sandbox.enabled=false" >&2
    fails=$((fails + 1))
else
    echo "  ok: no NetworkPolicy when sandbox.enabled=false"
fi

echo "== template: rest-connector without spec must fail"
if helm template "$RELEASE" "$CHART_DIR" --set restConnector.enabled=true >/dev/null 2>&1; then
    echo "  FAIL: expected 'required' error for missing restConnector.specUrl" >&2
    fails=$((fails + 1))
else
    echo "  ok: missing specUrl rejected"
fi

echo "== template: postgres disabled without externalDatabaseUrl must fail"
if helm template "$RELEASE" "$CHART_DIR" --set postgres.enabled=false >/dev/null 2>&1; then
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
