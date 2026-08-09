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
	SchemaGroup        = "urn:ietf:params:scim:schemas:core:2.0:Group"
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

// Member is one entry of a Group's member list. Only Value (the member's id)
// is load-bearing; Display and Ref are conveniences identity providers show in
// their UI, and Type distinguishes User members from nested Group members —
// which this gateway does not support.
type Member struct {
	Value   string `json:"value"`
	Display string `json:"display,omitempty"`
	Ref     string `json:"$ref,omitempty"`
	Type    string `json:"type,omitempty"`
}

// Group is the SCIM core Group resource (the subset the gateway serves).
//
// Members is a POINTER to a slice, not a slice, because three states must be
// distinguishable on the wire and only a pointer gives all three:
//
//	nil pointer            -> the key is omitted   (?excludedAttributes=members)
//	pointer to empty slice -> "members": []        (a group with no members)
//	pointer to a populated slice
//
// A plain []Member with omitempty collapses the middle case into the first,
// dropping the key for an empty group — which Okta treats as a malformed
// response. This is the same tri-state discipline the User path uses for
// *bool Active.
type Group struct {
	Schemas     []string  `json:"schemas"`
	ID          string    `json:"id"`
	ExternalID  string    `json:"externalId,omitempty"`
	DisplayName string    `json:"displayName"`
	Members     *[]Member `json:"members,omitempty"`
	Meta        Meta      `json:"meta"`
}

// NewGroup builds a Group resource. baseURL is the absolute /scim/v2/Groups
// prefix used for meta.location. members may be nil, which still yields
// "members": [] — callers that want the key omitted set Members to nil after
// construction, so omission is always a deliberate act rather than an accident
// of an empty result.
func NewGroup(id, externalID, displayName string, members []Member, baseURL string) Group {
	if members == nil {
		members = []Member{}
	}
	return Group{
		Schemas:     []string{SchemaGroup},
		ID:          id,
		ExternalID:  externalID,
		DisplayName: displayName,
		Members:     &members,
		Meta:        Meta{ResourceType: "Group", Location: strings.TrimRight(baseURL, "/") + "/" + id},
	}
}

// GroupListResponse is the ListResponse envelope for Groups. It duplicates
// ListResponse's fields rather than making that type generic: ListResponse is
// on the User path that identity providers already consume, and reshaping it
// to serve a second resource is a change to working code for no gain.
type GroupListResponse struct {
	Schemas      []string `json:"schemas"`
	TotalResults int      `json:"totalResults"`
	StartIndex   int      `json:"startIndex"`
	ItemsPerPage int      `json:"itemsPerPage"`
	Resources    []Group  `json:"Resources"`
}

// NewGroupListResponse wraps resources in a SCIM ListResponse. A nil slice is
// rendered as an empty JSON array.
func NewGroupListResponse(resources []Group) GroupListResponse {
	if resources == nil {
		resources = []Group{}
	}
	return GroupListResponse{
		Schemas:      []string{SchemaListResponse},
		TotalResults: len(resources),
		StartIndex:   1,
		ItemsPerPage: len(resources),
		Resources:    resources,
	}
}

// GroupOpKind is the operation a Group PATCH asks for.
type GroupOpKind int

const (
	GroupOpAddMembers GroupOpKind = iota
	GroupOpRemoveMembers
	GroupOpReplaceMembers
	GroupOpSetDisplayName
)

// GroupOp is one normalised Group PATCH operation.
type GroupOp struct {
	Kind        GroupOpKind
	MemberIDs   []string // for add/remove/replace; empty on a bare "remove members"
	DisplayName string   // for GroupOpSetDisplayName
}

// memberValue is the member entry shape inside a PATCH value array.
type memberValue struct {
	Value string `json:"value"`
}

// membersPathFilter extracts x from a `members[value eq "x"]` path. Okta removes
// a single member with this form rather than by sending a value array.
func membersPathFilter(path string) (id string, ok bool) {
	open := strings.Index(path, "[")
	if open < 0 || !strings.EqualFold(strings.TrimSpace(path[:open]), "members") {
		return "", false
	}
	if !strings.HasSuffix(path, "]") {
		return "", false
	}
	inner := strings.TrimSpace(path[open+1 : len(path)-1])
	fields := strings.SplitN(inner, " ", 3)
	if len(fields) != 3 || !strings.EqualFold(fields[0], "value") || !strings.EqualFold(fields[1], "eq") {
		return "", false
	}
	v := strings.TrimSpace(fields[2])
	if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') && v[len(v)-1] == v[0] {
		return v[1 : len(v)-1], true
	}
	return "", false
}

// GroupOps normalises a Group PATCH body into an ORDERED slice of operations.
//
// Order is preserved rather than fused into a single result because Okta packs
// a remove and an add into one Operations array: a member removed by the first
// op and re-added by the second must end up present, which only an ordered
// application gets right.
//
// Every verb and attribute is compared case-insensitively. Entra sends
// "Add"/"Replace"/"Remove" capitalised and documents that servers must not
// match case-sensitively; Okta sends them lowercase.
//
// Recognised forms:
//
//	{"op":"Add",    "path":"members","value":[{"value":"usr_x"}]}   add
//	{"op":"Remove", "path":"members","value":[{"value":"usr_x"}]}   remove those
//	{"op":"remove", "path":"members[value eq \"usr_x\"]"}           remove that one
//	{"op":"replace","path":"members","value":[...]}                 wholesale
//	{"op":"remove", "path":"members"}                               remove all
//	{"op":"replace","path":"displayName","value":"New"}             rename
//	{"op":"replace","value":{"displayName":"New"}}                  pathless rename
//
// Unrecognised operations are skipped rather than rejected: identity providers
// send attributes this gateway does not model, and failing the whole request
// would stall provisioning over an attribute nobody reads.
func (p PatchRequest) GroupOps() []GroupOp {
	ops := p.Operations
	if len(ops) == 0 && p.Op != "" {
		ops = []PatchOperation{{Op: p.Op, Path: p.Path, Value: p.Value}}
	}

	var out []GroupOp
	for _, op := range ops {
		path := strings.TrimSpace(op.Path)

		if id, ok := membersPathFilter(path); ok && strings.EqualFold(op.Op, "remove") {
			out = append(out, GroupOp{Kind: GroupOpRemoveMembers, MemberIDs: []string{id}})
			continue
		}

		if strings.EqualFold(path, "members") {
			var vals []memberValue
			_ = json.Unmarshal(op.Value, &vals) // absent/!array -> empty, which is the "all" case
			ids := make([]string, 0, len(vals))
			for _, v := range vals {
				if v.Value != "" {
					ids = append(ids, v.Value)
				}
			}
			switch {
			case strings.EqualFold(op.Op, "add"):
				out = append(out, GroupOp{Kind: GroupOpAddMembers, MemberIDs: ids})
			case strings.EqualFold(op.Op, "remove"):
				// No value array means remove every member (RFC 7644 §3.5.2).
				out = append(out, GroupOp{Kind: GroupOpRemoveMembers, MemberIDs: ids})
			case strings.EqualFold(op.Op, "replace"):
				out = append(out, GroupOp{Kind: GroupOpReplaceMembers, MemberIDs: ids})
			}
			continue
		}

		if strings.EqualFold(path, "displayName") &&
			(strings.EqualFold(op.Op, "replace") || strings.EqualFold(op.Op, "add")) {
			var name string
			if err := json.Unmarshal(op.Value, &name); err == nil && name != "" {
				out = append(out, GroupOp{Kind: GroupOpSetDisplayName, DisplayName: name})
			}
			continue
		}

		if path == "" && (strings.EqualFold(op.Op, "replace") || strings.EqualFold(op.Op, "add")) {
			// Pathless form: the value object carries the attributes.
			var obj struct {
				DisplayName string        `json:"displayName"`
				Members     []memberValue `json:"members"`
			}
			if err := json.Unmarshal(op.Value, &obj); err != nil {
				continue
			}
			if obj.DisplayName != "" {
				out = append(out, GroupOp{Kind: GroupOpSetDisplayName, DisplayName: obj.DisplayName})
			}
			if obj.Members != nil {
				ids := make([]string, 0, len(obj.Members))
				for _, v := range obj.Members {
					if v.Value != "" {
						ids = append(ids, v.Value)
					}
				}
				out = append(out, GroupOp{Kind: GroupOpReplaceMembers, MemberIDs: ids})
			}
		}
	}
	return out
}
