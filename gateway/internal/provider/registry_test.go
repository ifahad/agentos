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
