package scim

// Discovery documents (RFC 7643 §6) advertised by the Bearer-protected
// GET /scim/v2/ServiceProviderConfig, /ResourceTypes, and /Schemas endpoints.
// They are the minimal-but-valid subset an IdP (Okta/Entra) introspects: the
// User resource, filter support, and PATCH — no bulk, no sort, no ETag.

// ServiceProviderConfig returns the SCIM ServiceProviderConfig document. baseURL
// is the absolute /scim/v2 prefix used for documentationUri-adjacent links.
func ServiceProviderConfig() map[string]any {
	return map[string]any{
		"schemas":          []string{SchemaServiceProviderConfig},
		"documentationUri": "https://datatracker.ietf.org/doc/html/rfc7644",
		"patch":            map[string]any{"supported": true},
		"bulk":             map[string]any{"supported": false, "maxOperations": 0, "maxPayloadSize": 0},
		"filter":           map[string]any{"supported": true, "maxResults": 200},
		"changePassword":   map[string]any{"supported": false},
		"sort":             map[string]any{"supported": false},
		"etag":             map[string]any{"supported": false},
		"authenticationSchemes": []any{map[string]any{
			"type":        "oauthbearertoken",
			"name":        "OAuth Bearer Token",
			"description": "Authentication via the SCIM bearer token (AGENTOS_SCIM_TOKEN).",
			"primary":     true,
		}},
		"meta": map[string]any{"resourceType": "ServiceProviderConfig"},
	}
}

// ResourceTypes returns the SCIM ResourceTypes listing (just the User type).
func ResourceTypes() ListResponseRaw {
	user := map[string]any{
		"schemas":     []string{SchemaResourceType},
		"id":          "User",
		"name":        "User",
		"endpoint":    "/Users",
		"description": "User Account",
		"schema":      SchemaUser,
		"meta":        map[string]any{"resourceType": "ResourceType", "location": "/scim/v2/ResourceTypes/User"},
	}
	return newRawList([]any{user})
}

// Schemas returns the SCIM Schemas listing advertising the core User schema and
// the attributes the gateway honors.
func Schemas() ListResponseRaw {
	userSchema := map[string]any{
		"schemas":     []string{SchemaSchema},
		"id":          SchemaUser,
		"name":        "User",
		"description": "User Account",
		"attributes": []any{
			attr("userName", "string", true, "server"),
			attr("externalId", "string", false, "none"),
			attr("active", "boolean", false, "none"),
			complexAttr("name", []any{attr("formatted", "string", false, "none")}),
			complexAttr("emails", []any{attr("value", "string", false, "none"), attr("primary", "boolean", false, "none")}),
		},
		"meta": map[string]any{"resourceType": "Schema", "location": "/scim/v2/Schemas/" + SchemaUser},
	}
	return newRawList([]any{userSchema})
}

// ListResponseRaw is a SCIM ListResponse over arbitrary resource maps (used by
// the discovery listings, whose resources are not User records).
type ListResponseRaw struct {
	Schemas      []string `json:"schemas"`
	TotalResults int      `json:"totalResults"`
	StartIndex   int      `json:"startIndex"`
	ItemsPerPage int      `json:"itemsPerPage"`
	Resources    []any    `json:"Resources"`
}

func newRawList(resources []any) ListResponseRaw {
	return ListResponseRaw{
		Schemas:      []string{SchemaListResponse},
		TotalResults: len(resources),
		StartIndex:   1,
		ItemsPerPage: len(resources),
		Resources:    resources,
	}
}

func attr(name, typ string, required bool, uniqueness string) map[string]any {
	m := map[string]any{
		"name":        name,
		"type":        typ,
		"multiValued": false,
		"required":    required,
		"caseExact":   false,
		"mutability":  "readWrite",
		"returned":    "default",
		"uniqueness":  uniqueness,
	}
	return m
}

func complexAttr(name string, sub []any) map[string]any {
	multi := name == "emails"
	return map[string]any{
		"name":          name,
		"type":          "complex",
		"multiValued":   multi,
		"required":      false,
		"mutability":    "readWrite",
		"returned":      "default",
		"subAttributes": sub,
	}
}
