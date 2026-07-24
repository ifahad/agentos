package provider

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/ifahad/agentos/gateway/internal/safehttp"
)

func stubLookup(t *testing.T) {
	t.Helper()
	safehttp.SetLookupIPForTest(func(host string) ([]net.IP, error) {
		switch host {
		case "api.moonshot.ai", "open.bigmodel.cn":
			return []net.IP{net.ParseIP("203.0.113.10")}, nil
		case "metadata.internal":
			return []net.IP{net.ParseIP("169.254.169.254")}, nil
		}
		return nil, errors.New("no such host")
	})
	t.Cleanup(func() { safehttp.SetLookupIPForTest(net.LookupIP) })
}

func TestParseRegistryValid(t *testing.T) {
	stubLookup(t)
	data := []byte(`{"providers":[
	  {"name":"moonshot","base_url":"https://api.moonshot.ai",
	   "key_name":"AGENTOS_MOONSHOT_API_KEY","enabled":true,"max_attempts":3,
	   "prices":{"kimi-k3":{"in":1.0,"out":4.0}}}]}`)
	reg, errs := ParseRegistry(data)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	e, ok := reg.Lookup("moonshot")
	if !ok {
		t.Fatal("moonshot not found")
	}
	if e.BaseURL != "https://api.moonshot.ai" || e.KeyName != "AGENTOS_MOONSHOT_API_KEY" {
		t.Errorf("entry = %+v", e)
	}
	if e.MaxAttempts != 3 {
		t.Errorf("MaxAttempts = %d, want 3", e.MaxAttempts)
	}
	if p := e.Prices["kimi-k3"]; p.In != 1.0 || p.Out != 4.0 {
		t.Errorf("price = %+v", p)
	}
}

func TestParseRegistryRejects(t *testing.T) {
	stubLookup(t)
	cases := []struct{ name, json string }{
		{"private host", `{"providers":[{"name":"evil","base_url":"https://metadata.internal","key_name":"K"}]}`},
		{"literal metadata IP", `{"providers":[{"name":"evil","base_url":"https://169.254.169.254","key_name":"K"}]}`},
		{"plain http remote", `{"providers":[{"name":"evil","base_url":"http://api.moonshot.ai","key_name":"K"}]}`},
		{"missing name", `{"providers":[{"base_url":"https://api.moonshot.ai","key_name":"K"}]}`},
		{"name with slash", `{"providers":[{"name":"bad/name","base_url":"https://api.moonshot.ai","key_name":"K"}]}`},
		{"missing base_url", `{"providers":[{"name":"x","key_name":"K"}]}`},
		{"unparseable url", `{"providers":[{"name":"x","base_url":"://nope","key_name":"K"}]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reg, errs := ParseRegistry([]byte(tc.json))
			if len(errs) == 0 {
				t.Fatal("want a validation error, got none")
			}
			if _, ok := reg.Lookup("evil"); ok {
				t.Error("rejected entry must not be registered")
			}
			if _, ok := reg.Lookup("x"); ok {
				t.Error("rejected entry must not be registered")
			}
		})
	}
}

func TestParseRegistryAllowsLoopbackHTTP(t *testing.T) {
	stubLookup(t)
	data := []byte(`{"providers":[{"name":"local","base_url":"http://localhost:11434","key_name":""}]}`)
	reg, errs := ParseRegistry(data)
	if len(errs) != 0 {
		t.Fatalf("loopback http must be allowed, got %v", errs)
	}
	if _, ok := reg.Lookup("local"); !ok {
		t.Error("local not registered")
	}
}

func TestParseRegistryOneBadEntryDoesNotKillTheRest(t *testing.T) {
	stubLookup(t)
	data := []byte(`{"providers":[
	  {"name":"good","base_url":"https://api.moonshot.ai","key_name":"K"},
	  {"name":"evil","base_url":"https://metadata.internal","key_name":"K"}]}`)
	reg, errs := ParseRegistry(data)
	if len(errs) != 1 {
		t.Fatalf("errs = %v, want exactly 1", errs)
	}
	if _, ok := reg.Lookup("good"); !ok {
		t.Error("good entry must survive a sibling's rejection")
	}
}

func TestLoadRegistryMissingFileIsEmptyNotAnError(t *testing.T) {
	reg, errs := LoadRegistry(filepath.Join(t.TempDir(), "absent.json"))
	if len(errs) != 0 {
		t.Fatalf("missing file must not error, got %v", errs)
	}
	if len(reg.Names()) != 0 {
		t.Errorf("Names() = %v, want empty", reg.Names())
	}
}

func TestLoadRegistryReadsFile(t *testing.T) {
	stubLookup(t)
	path := filepath.Join(t.TempDir(), "providers.json")
	if err := os.WriteFile(path, []byte(`{"providers":[{"name":"moonshot","base_url":"https://api.moonshot.ai","key_name":"K"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	reg, errs := LoadRegistry(path)
	if len(errs) != 0 {
		t.Fatalf("errs = %v", errs)
	}
	if _, ok := reg.Lookup("moonshot"); !ok {
		t.Error("moonshot not loaded")
	}
}

func TestRouteViaRegistry(t *testing.T) {
	stubLookup(t)
	reg, errs := ParseRegistry([]byte(`{"providers":[
	  {"name":"moonshot","base_url":"https://api.moonshot.ai",
	   "key_name":"AGENTOS_MOONSHOT_API_KEY","enabled":true,
	   "prices":{"kimi-k3":{"in":1.0,"out":4.0}}}]}`))
	if len(errs) != 0 {
		t.Fatalf("errs = %v", errs)
	}
	router := &Router{Registry: reg, Secrets: staticSource{"AGENTOS_MOONSHOT_API_KEY": "sk-moon"}}

	route, err := router.Route("moonshot/kimi-k3")
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if route.Provider != "moonshot" {
		t.Errorf("Provider = %q", route.Provider)
	}
	if route.URL != "https://api.moonshot.ai/v1/chat/completions" {
		t.Errorf("URL = %q", route.URL)
	}
	if route.APIKey != "sk-moon" {
		t.Errorf("APIKey = %q", route.APIKey)
	}
	if route.Model != "kimi-k3" {
		t.Errorf("Model = %q", route.Model)
	}
	if route.MaxAttempts != DefaultMaxAttempts {
		t.Errorf("MaxAttempts = %d, want %d", route.MaxAttempts, DefaultMaxAttempts)
	}
	// 1M in + 1M out at 1.0/4.0 == $5.00 — budgets are no longer inert.
	if got := route.Cost(1_000_000, 1_000_000); got != 5.0 {
		t.Errorf("Cost = %v, want 5.0", got)
	}

	emb, err := router.RouteEmbeddings("moonshot/embed-1")
	if err != nil {
		t.Fatalf("RouteEmbeddings: %v", err)
	}
	if emb.URL != "https://api.moonshot.ai/v1/embeddings" {
		t.Errorf("embeddings URL = %q", emb.URL)
	}
	if got := emb.Cost(1_000_000, 0); got != 0 {
		t.Errorf("unpriced model must cost 0, got %v", got)
	}
}

func TestRouteRegistryDoesNotShadowBuiltins(t *testing.T) {
	stubLookup(t)
	reg, _ := ParseRegistry([]byte(`{"providers":[
	  {"name":"anthropic","base_url":"https://api.moonshot.ai","key_name":"K"}]}`))
	router := &Router{Registry: reg, AnthropicAPIKey: "sk-ant"}
	route, err := router.Route("anthropic/claude-sonnet-5")
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if route.URL != "https://api.anthropic.com/v1/chat/completions" {
		t.Errorf("built-in must win over a registry entry of the same name, got %q", route.URL)
	}
}

func TestRouteDisabledProviderIsUnknown(t *testing.T) {
	stubLookup(t)
	reg, _ := ParseRegistry([]byte(`{"providers":[
	  {"name":"moonshot","base_url":"https://api.moonshot.ai","key_name":"K","enabled":false}]}`))
	router := &Router{Registry: reg}
	if _, err := router.Route("moonshot/kimi-k3"); !errors.Is(err, ErrUnknownProvider) {
		t.Errorf("err = %v, want ErrUnknownProvider", err)
	}
}

func TestRouteEnabledProviderWithNoKeyIsUnknown(t *testing.T) {
	stubLookup(t)
	reg, _ := ParseRegistry([]byte(`{"providers":[
	  {"name":"moonshot","base_url":"https://api.moonshot.ai",
	   "key_name":"AGENTOS_MOONSHOT_API_KEY","enabled":true}]}`))
	router := &Router{Registry: reg, Secrets: staticSource{}}
	if _, err := router.Route("moonshot/kimi-k3"); !errors.Is(err, ErrUnknownProvider) {
		t.Errorf("a provider with no resolvable key must not route, got err = %v", err)
	}
}

func TestBuiltinPricingStillApplies(t *testing.T) {
	router := &Router{}
	route, err := router.Route("anthropic/claude-sonnet-5")
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if got := route.Cost(1_000_000, 1_000_000); got != 18.0 {
		t.Errorf("Cost = %v, want 18.0 (3 in + 15 out)", got)
	}
	unknown, _ := router.Route("ollama/qwen3.6:latest")
	if got := unknown.Cost(1_000_000, 1_000_000); got != 0 {
		t.Errorf("local model must cost 0, got %v", got)
	}
}

// staticSource is a secret.Source backed by a map.
type staticSource map[string]string

func (s staticSource) Get(name string) (string, bool) { v, ok := s[name]; return v, ok }
func (s staticSource) Backend() string                { return "static" }
