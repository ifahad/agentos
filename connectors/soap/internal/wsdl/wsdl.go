// Package wsdl loads and parses a WSDL 1.1 document into the flat operation
// list the agentos-soap connector exposes as MCP tools. Parsing is deliberately
// minimal (plain encoding/xml structs, no third-party SOAP library): only the
// service endpoint (soap:address), portType operations, binding SOAPAction
// values, and input message parts (resolved through an inline schema to their
// top-level element/part names) are read.
//
// SOAP 1.1 (soap namespace http://schemas.xmlsoap.org/wsdl/soap/) is handled
// first; a SOAP 1.2 binding (soap12 namespace) is detected and logged but the
// connector always builds SOAP 1.1 envelopes.
package wsdl

import (
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"unicode"
)

// SOAP binding namespaces used to tell the two WSDL SOAP bindings apart.
const (
	soap11NS = "http://schemas.xmlsoap.org/wsdl/soap/"
	soap12NS = "http://schemas.xmlsoap.org/wsdl/soap12/"
)

// Operation is one WSDL operation, ready to be registered as an MCP tool.
type Operation struct {
	// Name is the WSDL operation name (e.g. "Add").
	Name string
	// ToolName is Name snake_cased (e.g. "add").
	ToolName string
	// SOAPAction is the binding's soapAction value, sent as the SOAPAction
	// header (quoted).
	SOAPAction string
	// Doc is the operation's <documentation>, if any.
	Doc string
	// Element is the request body wrapper element's local name. For
	// document/literal-wrapped services this is the element referenced by the
	// input message part (usually == Name); for RPC style it is Name.
	Element string
	// Namespace is the xmlns applied to the wrapper element.
	Namespace string
	// Parts are the flat, top-level input part/argument names, in document
	// order — one string arg per part.
	Parts []string
}

// Definition is the parsed WSDL: the chosen service endpoint plus the flat
// operation list.
type Definition struct {
	Service        string
	Endpoint       string
	TargetNS       string
	SOAPVersion    string // "1.1" or "1.2" — the binding the operations came from
	SOAP12Detected bool   // a SOAP 1.2 binding was present in the document
	Operations     []Operation
}

// ---- raw encoding/xml shapes ---------------------------------------------
//
// Field tags carry local names only, so elements match regardless of the
// namespace prefix the document happens to use. The one place the namespace
// matters (SOAP 1.1 vs 1.2) is captured explicitly via xml.Name fields.

type rawDefinitions struct {
	XMLName         xml.Name      `xml:"definitions"`
	TargetNamespace string        `xml:"targetNamespace,attr"`
	Types           rawTypes      `xml:"types"`
	Messages        []rawMessage  `xml:"message"`
	PortTypes       []rawPortType `xml:"portType"`
	Bindings        []rawBinding  `xml:"binding"`
	Services        []rawService  `xml:"service"`
}

type rawTypes struct {
	Schemas []rawSchema `xml:"schema"`
}

type rawSchema struct {
	TargetNamespace string       `xml:"targetNamespace,attr"`
	Elements        []rawElement `xml:"element"`
}

type rawElement struct {
	Name        string         `xml:"name,attr"`
	Type        string         `xml:"type,attr"`
	ComplexType rawComplexType `xml:"complexType"`
}

type rawComplexType struct {
	Sequence rawSequence `xml:"sequence"`
}

type rawSequence struct {
	Elements []rawSeqElement `xml:"element"`
}

type rawSeqElement struct {
	Name string `xml:"name,attr"`
	Type string `xml:"type,attr"`
}

type rawMessage struct {
	Name  string    `xml:"name,attr"`
	Parts []rawPart `xml:"part"`
}

type rawPart struct {
	Name    string `xml:"name,attr"`
	Element string `xml:"element,attr"`
	Type    string `xml:"type,attr"`
}

type rawPortType struct {
	Name       string           `xml:"name,attr"`
	Operations []rawPTOperation `xml:"operation"`
}

type rawPTOperation struct {
	Name          string `xml:"name,attr"`
	Documentation string `xml:"documentation"`
	Input         rawIO  `xml:"input"`
	Output        rawIO  `xml:"output"`
}

type rawIO struct {
	Message string `xml:"message,attr"`
	Name    string `xml:"name,attr"`
}

type rawBinding struct {
	Name        string             `xml:"name,attr"`
	Type        string             `xml:"type,attr"`
	SOAPBinding rawNamespacedEmpty `xml:"binding"`
	Operations  []rawBindOperation `xml:"operation"`
}

// rawNamespacedEmpty captures an element only for its namespace (soap:binding),
// which distinguishes a SOAP 1.1 binding from a SOAP 1.2 one.
type rawNamespacedEmpty struct {
	XMLName xml.Name
}

type rawBindOperation struct {
	Name          string           `xml:"name,attr"`
	SOAPOperation rawSOAPOperation `xml:"operation"`
}

type rawSOAPOperation struct {
	XMLName    xml.Name
	SOAPAction string `xml:"soapAction,attr"`
}

type rawService struct {
	Name  string    `xml:"name,attr"`
	Ports []rawPort `xml:"port"`
}

type rawPort struct {
	Name    string         `xml:"name,attr"`
	Binding string         `xml:"binding,attr"`
	Address rawSOAPAddress `xml:"address"`
}

type rawSOAPAddress struct {
	XMLName  xml.Name
	Location string `xml:"location,attr"`
}

// ---- loading --------------------------------------------------------------

// Load fetches the WSDL from wsdlURL — http(s):// or a local file path — and
// parses it.
func Load(wsdlURL string) (*Definition, error) {
	data, err := fetch(wsdlURL)
	if err != nil {
		return nil, err
	}
	return Parse(data)
}

func fetch(wsdlURL string) ([]byte, error) {
	if strings.HasPrefix(wsdlURL, "http://") || strings.HasPrefix(wsdlURL, "https://") {
		resp, err := http.Get(wsdlURL)
		if err != nil {
			return nil, fmt.Errorf("fetch wsdl %s: %w", wsdlURL, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("fetch wsdl %s: unexpected status %d", wsdlURL, resp.StatusCode)
		}
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("read wsdl %s: %w", wsdlURL, err)
		}
		return data, nil
	}
	data, err := os.ReadFile(wsdlURL)
	if err != nil {
		return nil, fmt.Errorf("read wsdl file: %w", err)
	}
	return data, nil
}

// schemaElement is a resolved top-level schema element: its target namespace
// plus the child part names of its (wrapped) complexType sequence.
type schemaElement struct {
	namespace string
	parts     []string
}

// Parse decodes a WSDL 1.1 document into a Definition. It prefers the SOAP 1.1
// binding; if only a SOAP 1.2 binding is present it logs a note and uses that
// binding's operations (envelopes are still built as SOAP 1.1). Operations are
// returned sorted by name so registration order is deterministic.
func Parse(data []byte) (*Definition, error) {
	var raw rawDefinitions
	if err := xml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse wsdl: %w", err)
	}

	// Index top-level schema elements by local name.
	elements := map[string]schemaElement{}
	for _, sc := range raw.Types.Schemas {
		for _, el := range sc.Elements {
			if el.Name == "" {
				continue
			}
			parts := make([]string, 0, len(el.ComplexType.Sequence.Elements))
			for _, child := range el.ComplexType.Sequence.Elements {
				if child.Name != "" {
					parts = append(parts, child.Name)
				}
			}
			elements[el.Name] = schemaElement{namespace: sc.TargetNamespace, parts: parts}
		}
	}

	// Index messages by local name.
	messages := map[string]rawMessage{}
	for _, m := range raw.Messages {
		messages[local(m.Name)] = m
	}

	// Index portType operations by name (documentation + input message).
	ptOps := map[string]rawPTOperation{}
	for _, pt := range raw.PortTypes {
		for _, op := range pt.Operations {
			ptOps[op.Name] = op
		}
	}

	// Choose the binding: prefer SOAP 1.1, note any SOAP 1.2 present.
	var chosen *rawBinding
	chosenVersion := ""
	soap12Detected := false
	for i := range raw.Bindings {
		b := &raw.Bindings[i]
		switch b.SOAPBinding.XMLName.Space {
		case soap11NS:
			if chosen == nil || chosenVersion != "1.1" {
				chosen = b
				chosenVersion = "1.1"
			}
		case soap12NS:
			soap12Detected = true
			if chosen == nil {
				chosen = b
				chosenVersion = "1.2"
			}
		}
	}
	if chosen == nil {
		return nil, fmt.Errorf("parse wsdl: no SOAP binding found (neither %s nor %s)", soap11NS, soap12NS)
	}
	if soap12Detected && chosenVersion == "1.1" {
		log.Print("soap-connector: SOAP 1.2 binding detected; using the SOAP 1.1 binding (1.2 not exposed)")
	}
	if chosenVersion == "1.2" {
		log.Print("soap-connector: only a SOAP 1.2 binding was found; exposing it but building SOAP 1.1 envelopes (best effort)")
	}

	def := &Definition{
		TargetNS:       raw.TargetNamespace,
		SOAPVersion:    chosenVersion,
		SOAP12Detected: soap12Detected,
	}

	// Endpoint: the soap:address of the service port bound to the chosen
	// binding (fall back to the first port with an address).
	def.Endpoint, def.Service = endpointFor(raw.Services, chosen.Name)

	for _, bop := range chosen.Operations {
		pt := ptOps[bop.Name]
		element, ns, parts := resolveInput(bop.Name, pt.Input.Message, messages, elements, raw.TargetNamespace)
		def.Operations = append(def.Operations, Operation{
			Name:       bop.Name,
			ToolName:   SnakeCase(bop.Name),
			SOAPAction: bop.SOAPOperation.SOAPAction,
			Doc:        strings.TrimSpace(pt.Documentation),
			Element:    element,
			Namespace:  ns,
			Parts:      parts,
		})
	}

	sort.Slice(def.Operations, func(i, j int) bool {
		return def.Operations[i].Name < def.Operations[j].Name
	})
	return def, nil
}

// endpointFor returns the soap:address location (and service name) of the port
// bound to bindingName, falling back to the first port that carries an address.
func endpointFor(services []rawService, bindingName string) (endpoint, service string) {
	var fallback, fallbackSvc string
	for _, svc := range services {
		for _, p := range svc.Ports {
			if p.Address.Location == "" {
				continue
			}
			if fallback == "" {
				fallback, fallbackSvc = p.Address.Location, svc.Name
			}
			if local(p.Binding) == bindingName {
				return p.Address.Location, svc.Name
			}
		}
	}
	return fallback, fallbackSvc
}

// resolveInput turns an operation's input message into the request wrapper
// element name, its namespace, and the flat list of part/argument names.
//
//   - document/literal wrapped: the message part references a schema element
//     (e.g. tns:Add); the wrapper element is that element and the parts are the
//     children of its complexType sequence.
//   - RPC style: the message parts reference types directly; the wrapper
//     element is the operation name and the parts are the message part names.
func resolveInput(opName, messageRef string, messages map[string]rawMessage, elements map[string]schemaElement, targetNS string) (element, namespace string, parts []string) {
	msg, ok := messages[local(messageRef)]
	if !ok {
		return opName, targetNS, nil
	}

	for _, p := range msg.Parts {
		if p.Element != "" {
			elemLocal := local(p.Element)
			ns := targetNS
			var ps []string
			if se, ok := elements[elemLocal]; ok {
				if se.namespace != "" {
					ns = se.namespace
				}
				ps = append(ps, se.parts...)
			}
			return elemLocal, ns, ps
		}
	}

	// RPC style: parts are the arguments, wrapper element is the operation.
	for _, p := range msg.Parts {
		if p.Name != "" {
			parts = append(parts, p.Name)
		}
	}
	return opName, targetNS, parts
}

// local strips a namespace prefix ("tns:Add" -> "Add").
func local(qname string) string {
	if i := strings.IndexByte(qname, ':'); i >= 0 {
		return qname[i+1:]
	}
	return qname
}

// SnakeCase converts an operation name to a snake_case tool name: CamelCase
// becomes camel_case (acronym runs are kept together: HTTPGet -> http_get) and
// dashes, dots and spaces become underscores.
func SnakeCase(s string) string {
	runes := []rune(s)
	var b strings.Builder
	for i, r := range runes {
		switch {
		case r == '-' || r == '.' || r == ' ':
			b.WriteRune('_')
		case unicode.IsUpper(r):
			if i > 0 {
				prev := runes[i-1]
				nextLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
				if unicode.IsLower(prev) || unicode.IsDigit(prev) || (unicode.IsUpper(prev) && nextLower) {
					b.WriteRune('_')
				}
			}
			b.WriteRune(unicode.ToLower(r))
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
