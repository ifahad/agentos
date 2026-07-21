// Package scim implements the SCIM 2.0 (RFC 7643/7644) JSON shapes used by the
// gateway's /scim/v2/* provisioning API: the User resource, the Error message,
// and the ListResponse envelope, plus parsing of the PATCH request body. Only
// the subset the gateway supports (User create/read/list/replace/patch/delete
// and filter=userName eq) is modeled — no bulk, no sort.
package scim

import (
	"encoding/json"
	"strings"
)

// Schema URNs from RFC 7643/7644.
const (
	SchemaUser         = "urn:ietf:params:scim:schemas:core:2.0:User"
	SchemaError        = "urn:ietf:params:scim:api:messages:2.0:Error"
	SchemaListResponse = "urn:ietf:params:scim:api:messages:2.0:ListResponse"
	SchemaPatchOp      = "urn:ietf:params:scim:api:messages:2.0:PatchOp"

	SchemaServiceProviderConfig = "urn:ietf:params:scim:schemas:core:2.0:ServiceProviderConfig"
	SchemaResourceType          = "urn:ietf:params:scim:schemas:core:2.0:ResourceType"
	SchemaSchema                = "urn:ietf:params:scim:schemas:core:2.0:Schema"
)

// Name is the SCIM complex name attribute; only formatted is modeled.
type Name struct {
	Formatted string `json:"formatted,omitempty"`
}

// Email is one entry of the SCIM emails multi-valued attribute.
type Email struct {
	Value   string `json:"value"`
	Primary bool   `json:"primary"`
}

// Meta is the SCIM common metadata attribute.
type Meta struct {
	ResourceType string `json:"resourceType"`
	Location     string `json:"location,omitempty"`
}

// User is the SCIM core User resource (the subset the gateway serves).
type User struct {
	Schemas    []string `json:"schemas"`
	ID         string   `json:"id"`
	ExternalID string   `json:"externalId,omitempty"`
	UserName   string   `json:"userName"`
	Name       *Name    `json:"name,omitempty"`
	Emails     []Email  `json:"emails"`
	Active     bool     `json:"active"`
	Meta       Meta     `json:"meta"`
}

// NewUser builds a User resource for the given identity. baseURL is the
// absolute /scim/v2/Users prefix used for the meta.location link. formatted may
// be empty (the name attribute is then omitted).
func NewUser(id, externalID, userName, formatted string, active bool, baseURL string) User {
	u := User{
		Schemas:    []string{SchemaUser},
		ID:         id,
		ExternalID: externalID,
		UserName:   userName,
		Emails:     []Email{{Value: userName, Primary: true}},
		Active:     active,
		Meta:       Meta{ResourceType: "User", Location: strings.TrimRight(baseURL, "/") + "/" + id},
	}
	if formatted != "" {
		u.Name = &Name{Formatted: formatted}
	}
	return u
}

// Error is the SCIM error message shape. Status is the HTTP status as a string
// per RFC 7644 §3.12.
type Error struct {
	Schemas []string `json:"schemas"`
	Status  string   `json:"status"`
	Detail  string   `json:"detail"`
	// ScimType is an optional detailed error keyword (e.g. "uniqueness").
	ScimType string `json:"scimType,omitempty"`
}

// NewError builds a SCIM Error for the given HTTP status and detail.
func NewError(status int, detail string) Error {
	return Error{Schemas: []string{SchemaError}, Status: itoa(status), Detail: detail}
}

// NewErrorType builds a SCIM Error carrying a scimType keyword.
func NewErrorType(status int, scimType, detail string) Error {
	e := NewError(status, detail)
	e.ScimType = scimType
	return e
}

// ListResponse is the SCIM list envelope for a GET /Users query.
type ListResponse struct {
	Schemas      []string `json:"schemas"`
	TotalResults int      `json:"totalResults"`
	StartIndex   int      `json:"startIndex"`
	ItemsPerPage int      `json:"itemsPerPage"`
	Resources    []User   `json:"Resources"`
}

// NewListResponse wraps resources in a SCIM ListResponse. A nil slice is
// rendered as an empty JSON array.
func NewListResponse(resources []User) ListResponse {
	if resources == nil {
		resources = []User{}
	}
	return ListResponse{
		Schemas:      []string{SchemaListResponse},
		TotalResults: len(resources),
		StartIndex:   1,
		ItemsPerPage: len(resources),
		Resources:    resources,
	}
}

// PatchRequest is a SCIM PATCH body (RFC 7644 §3.5.2).
type PatchRequest struct {
	Schemas    []string         `json:"schemas"`
	Operations []PatchOperation `json:"Operations"`
	// Some clients (and the pathless form) send a single top-level operation
	// rather than an Operations array; these mirror PatchOperation.
	Op    string          `json:"op"`
	Path  string          `json:"path"`
	Value json.RawMessage `json:"value"`
}

// PatchOperation is one entry of a SCIM PatchRequest's Operations array.
type PatchOperation struct {
	Op    string          `json:"op"`
	Path  string          `json:"path"`
	Value json.RawMessage `json:"value"`
}

// ActivePatch scans a PATCH body for an operation that replaces the active
// attribute and returns the requested value. It accepts both supported forms:
//
//	{"Operations":[{"op":"replace","path":"active","value":false}]}   // path form
//	{"op":"replace","value":{"active":false}}                         // pathless form
//
// (the pathless value object form is also accepted inside an Operations entry).
// found is false when no active replace was present.
func (p PatchRequest) ActivePatch() (active bool, found bool) {
	ops := p.Operations
	// A single top-level op (no Operations array) is treated as one operation.
	if len(ops) == 0 && p.Op != "" {
		ops = []PatchOperation{{Op: p.Op, Path: p.Path, Value: p.Value}}
	}
	for _, op := range ops {
		if !strings.EqualFold(op.Op, "replace") && !strings.EqualFold(op.Op, "add") {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(op.Path), "active") {
			// Path form: value is the bare boolean.
			var v bool
			if err := json.Unmarshal(op.Value, &v); err == nil {
				return v, true
			}
			continue
		}
		if op.Path == "" {
			// Pathless form: value is an object carrying the attributes.
			var obj struct {
				Active *bool `json:"active"`
			}
			if err := json.Unmarshal(op.Value, &obj); err == nil && obj.Active != nil {
				return *obj.Active, true
			}
		}
	}
	return false, false
}

// itoa renders a small non-negative int without importing strconv into callers.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [12]byte
	i := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
