package wsdl

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func loadFixture(t *testing.T) *Definition {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "calculator.wsdl"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	def, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return def
}

func TestParseEndpointAndVersion(t *testing.T) {
	def := loadFixture(t)
	if def.Endpoint != "http://www.dneonline.com/calculator.asmx" {
		t.Errorf("Endpoint = %q, want the soap:address location", def.Endpoint)
	}
	if def.SOAPVersion != "1.1" {
		t.Errorf("SOAPVersion = %q, want 1.1 (must prefer the SOAP 1.1 binding)", def.SOAPVersion)
	}
	if !def.SOAP12Detected {
		t.Error("SOAP12Detected = false, want true (fixture has a soap12 binding)")
	}
	if def.TargetNS != "http://tempuri.org/" {
		t.Errorf("TargetNS = %q, want http://tempuri.org/", def.TargetNS)
	}
}

func TestParseOperations(t *testing.T) {
	def := loadFixture(t)

	want := []Operation{
		{
			Name: "Add", ToolName: "add", SOAPAction: "http://tempuri.org/Add",
			Doc: "Adds two integers.", Element: "Add", Namespace: "http://tempuri.org/",
			Parts: []string{"intA", "intB"},
		},
		{
			Name: "Divide", ToolName: "divide", SOAPAction: "http://tempuri.org/Divide",
			Doc: "Divides intA by intB.", Element: "Divide", Namespace: "http://tempuri.org/",
			Parts: []string{"intA", "intB"},
		},
		{
			Name: "Subtract", ToolName: "subtract", SOAPAction: "http://tempuri.org/Subtract",
			Doc: "Subtracts intB from intA.", Element: "Subtract", Namespace: "http://tempuri.org/",
			Parts: []string{"intA", "intB"},
		},
	}
	if !reflect.DeepEqual(def.Operations, want) {
		t.Errorf("Operations mismatch:\n got %+v\nwant %+v", def.Operations, want)
	}
}

func TestParseDeterministicOrdering(t *testing.T) {
	def := loadFixture(t)
	// Fixture declares Add, Subtract, Divide; parse must sort by name.
	var names []string
	for _, op := range def.Operations {
		names = append(names, op.Name)
	}
	want := []string{"Add", "Divide", "Subtract"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("operation order = %v, want %v (sorted)", names, want)
	}
}

// TestParseRPCStyle covers a WSDL whose message parts reference types directly
// (rpc/encoded) rather than a wrapped element, plus a single SOAP 1.1 binding.
func TestParseRPCStyle(t *testing.T) {
	const rpc = `<?xml version="1.0"?>
<definitions xmlns="http://schemas.xmlsoap.org/wsdl/"
             xmlns:soap="http://schemas.xmlsoap.org/wsdl/soap/"
             xmlns:tns="urn:calc" targetNamespace="urn:calc">
  <message name="AddIn">
    <part name="a" type="xsd:int"/>
    <part name="b" type="xsd:int"/>
  </message>
  <portType name="CalcPort">
    <operation name="Add">
      <documentation>rpc add</documentation>
      <input message="tns:AddIn"/>
    </operation>
  </portType>
  <binding name="CalcBinding" type="tns:CalcPort">
    <soap:binding transport="http://schemas.xmlsoap.org/soap/http"/>
    <operation name="Add">
      <soap:operation soapAction="urn:calc#Add"/>
    </operation>
  </binding>
  <service name="Calc">
    <port name="CalcPort" binding="tns:CalcBinding">
      <soap:address location="http://example/calc"/>
    </port>
  </service>
</definitions>`

	def, err := Parse([]byte(rpc))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if def.SOAP12Detected {
		t.Error("SOAP12Detected = true, want false")
	}
	if len(def.Operations) != 1 {
		t.Fatalf("got %d operations, want 1", len(def.Operations))
	}
	op := def.Operations[0]
	if op.Name != "Add" || op.SOAPAction != "urn:calc#Add" || op.Doc != "rpc add" {
		t.Errorf("unexpected op %+v", op)
	}
	if op.Element != "Add" || op.Namespace != "urn:calc" {
		t.Errorf("rpc wrapper = %q ns %q, want Add / urn:calc", op.Element, op.Namespace)
	}
	if !reflect.DeepEqual(op.Parts, []string{"a", "b"}) {
		t.Errorf("Parts = %v, want [a b]", op.Parts)
	}
	if def.Endpoint != "http://example/calc" {
		t.Errorf("Endpoint = %q", def.Endpoint)
	}
}

func TestParseInvalidXML(t *testing.T) {
	if _, err := Parse([]byte("<definitions")); err == nil {
		t.Fatal("Parse of invalid XML succeeded, want error")
	}
}

func TestParseNoSOAPBinding(t *testing.T) {
	const noSoap = `<?xml version="1.0"?>
<definitions xmlns="http://schemas.xmlsoap.org/wsdl/" targetNamespace="urn:x">
  <binding name="B" type="tns:P"><operation name="Op"/></binding>
</definitions>`
	if _, err := Parse([]byte(noSoap)); err == nil {
		t.Fatal("Parse without a SOAP binding succeeded, want error")
	}
}

func TestSnakeCase(t *testing.T) {
	tests := []struct{ in, want string }{
		{"Add", "add"},
		{"GetWeatherByZip", "get_weather_by_zip"},
		{"HTTPGet", "http_get"},
		{"getHTTPStatus", "get_http_status"},
		{"already_snake", "already_snake"},
		{"Convert-Temp", "convert_temp"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := SnakeCase(tt.in); got != tt.want {
			t.Errorf("SnakeCase(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestLoadFromHTTP(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "calculator.wsdl"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/xml")
		_, _ = w.Write(data)
	}))
	defer srv.Close()

	def, err := Load(srv.URL)
	if err != nil {
		t.Fatalf("Load(%s): %v", srv.URL, err)
	}
	if len(def.Operations) != 3 {
		t.Errorf("got %d operations, want 3", len(def.Operations))
	}
}

func TestLoadFromHTTPErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", http.StatusInternalServerError)
	}))
	defer srv.Close()
	if _, err := Load(srv.URL); err == nil {
		t.Fatal("Load from 500 endpoint succeeded, want error")
	}
}

func TestLoadFromFile(t *testing.T) {
	def, err := Load(filepath.Join("testdata", "calculator.wsdl"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(def.Operations) != 3 {
		t.Errorf("got %d operations, want 3", len(def.Operations))
	}
}

func TestLoadMissingFile(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "absent.wsdl")); err == nil {
		t.Fatal("Load of missing file succeeded, want error")
	}
}
