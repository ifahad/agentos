# agentos-soap connector

MCP server (`agentos-soap`, StreamableHTTP `/mcp` on `:8093`) that turns a
WSDL 1.1 document into tools: `list_operations` plus one tool per allowed SOAP
operation (name = operation snake_cased). Each tool builds a SOAP 1.1 envelope,
POSTs it to the service endpoint with the `SOAPAction` header, and returns the
response.

## Environment

| Variable | Default | Meaning |
| --- | --- | --- |
| `AGENTOS_SOAP_WSDL_URL` | — (required) | WSDL 1.1 document, `http(s)://` URL or file path. Fatal if unset or unfetchable. |
| `AGENTOS_SOAP_ENDPOINT` | WSDL `soap:address` | Upstream SOAP endpoint override. Fatal if both this and the WSDL address are absent. |
| `AGENTOS_SOAP_ALLOW_OPERATIONS` | — (all) | Comma-separated allowlist of operation names; empty exposes every operation. |
| `AGENTOS_SOAP_AUTH_HEADER` | — | Raw header `Name: value` attached to every upstream request. |
| `AGENTOS_SOAP_TIMEOUT_S` | `20` | Upstream request timeout, seconds. |
| `AGENTOS_SOAP_MAX_BODY_BYTES` | `131072` | Upstream response body cap; larger bodies are truncated (`"truncated": true`). |

## Tools

- `list_operations()` → JSON array of `{name, soap_action, doc}`.
- one tool per allowed operation. Its arguments are a **string per top-level
  input part** (flat) plus an `xml_body` escape hatch (see below). Result:
  `{"status": N, "body": ...}` where `body` is a parsed structure when the
  response is XML, otherwise a raw string (truncated at the cap).

### The `xml_body` escape hatch

Flat string args only cover simple, wrapped document/literal inputs. For nested
or complex types, pass `xml_body`: its value is used **verbatim** as the raw
inner body of the operation element and the per-part args are ignored. For
example, for an `Add` operation you may pass `intA`/`intB` args, **or** pass
`xml_body` = `<intA>3</intA><intB>4</intB>` (equivalent), or richer nested XML
that the flat args cannot express.

The envelope produced is:

```xml
<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/">
  <soapenv:Body>
    <Add xmlns="http://tempuri.org/">
      <intA>3</intA><intB>4</intB>   <!-- or the raw xml_body -->
    </Add>
  </soapenv:Body>
</soapenv:Envelope>
```

A SOAP **Fault** in the response is returned like any other body (status +
parsed fault), not raised as a tool error.

## Scope

**SOAP 1.1 only.** The WSDL is parsed with `encoding/xml` (no third-party SOAP
library): `soap:address` endpoint, `portType` operations, binding `soapAction`
values, and input message parts resolved through an inline schema to their
top-level element/part names. The connector prefers the WSDL's SOAP 1.1 binding
(namespace `http://schemas.xmlsoap.org/wsdl/soap/`); a SOAP 1.2 binding is
detected and logged but not exposed — envelopes are always built as SOAP 1.1.
