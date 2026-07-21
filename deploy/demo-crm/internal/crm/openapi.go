package crm

// openAPISpec is the hand-written OpenAPI 3 document served at /openapi.json.
// The agentos-rest connector parses it into MCP tools, so operationIds and
// parameter definitions must stay accurate.
const openAPISpec = `{
  "openapi": "3.0.3",
  "info": {
    "title": "Demo CRM",
    "description": "Tiny legacy CRM API for the AgentOS Phase 3 demo. Read-only static data consistent with the legacy ERP database.",
    "version": "1.0.0"
  },
  "servers": [
    {"url": "http://demo-crm:8095"}
  ],
  "paths": {
    "/customers": {
      "get": {
        "operationId": "listCustomers",
        "summary": "List CRM customers, optionally filtered by city (case-insensitive)",
        "parameters": [
          {
            "name": "city",
            "in": "query",
            "required": false,
            "description": "Filter by city, case-insensitive (e.g. Riyadh, Jeddah)",
            "schema": {"type": "string"}
          }
        ],
        "responses": {
          "200": {
            "description": "Array of {id, name, city, segment} customer objects",
            "content": {"application/json": {"schema": {"type": "array", "items": {"type": "object"}}}}
          }
        }
      }
    },
    "/customers/{id}": {
      "get": {
        "operationId": "getCustomer",
        "summary": "Get one customer by numeric id",
        "parameters": [
          {
            "name": "id",
            "in": "path",
            "required": true,
            "description": "Customer id (matches the legacy ERP customers.id)",
            "schema": {"type": "string"}
          }
        ],
        "responses": {
          "200": {
            "description": "The {id, name, city, segment} customer object",
            "content": {"application/json": {"schema": {"type": "object"}}}
          },
          "404": {
            "description": "Unknown customer id",
            "content": {"application/json": {"schema": {"type": "object"}}}
          }
        }
      }
    },
    "/tickets": {
      "get": {
        "operationId": "listTickets",
        "summary": "List support tickets, optionally filtered by status (open or closed)",
        "parameters": [
          {
            "name": "status",
            "in": "query",
            "required": false,
            "description": "Filter by ticket status: open or closed",
            "schema": {"type": "string", "enum": ["open", "closed"]}
          }
        ],
        "responses": {
          "200": {
            "description": "Array of {id, customer_id, subject, status, opened_at} ticket objects",
            "content": {"application/json": {"schema": {"type": "array", "items": {"type": "object"}}}}
          },
          "400": {
            "description": "Invalid status value",
            "content": {"application/json": {"schema": {"type": "object"}}}
          }
        }
      }
    }
  }
}
`
