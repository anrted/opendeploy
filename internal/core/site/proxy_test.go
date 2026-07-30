package site

import (
	"encoding/json"
	"testing"
)

func TestSiteProxyJSON(t *testing.T) {
	// Создать сайт без reverse proxy
	s1 := Site{
		ProxyEnabled: false,
	}

	b1, err := json.Marshal(s1)
	if err != nil {
		t.Fatalf("marshal site 1 error: %v", err)
	}

	var res1 map[string]interface{}
	if err := json.Unmarshal(b1, &res1); err != nil {
		t.Fatalf("unmarshal site 1 error: %v", err)
	}

	if enabled, ok := res1["proxy_enabled"].(bool); !ok || enabled != false {
		t.Errorf("site 1 proxy_enabled = %v, want false", res1["proxy_enabled"])
	}

	// Создать сайт с reverse proxy
	s2 := Site{
		ProxyEnabled: true,
		ProxyPort:    8080,
	}

	b2, err := json.Marshal(s2)
	if err != nil {
		t.Fatalf("marshal site 2 error: %v", err)
	}

	var res2 map[string]interface{}
	if err := json.Unmarshal(b2, &res2); err != nil {
		t.Fatalf("unmarshal site 2 error: %v", err)
	}

	if enabled, ok := res2["proxy_enabled"].(bool); !ok || enabled != true {
		t.Errorf("site 2 proxy_enabled = %v, want true", res2["proxy_enabled"])
	}

	if port, ok := res2["proxy_port"].(float64); !ok || int(port) != 8080 {
		t.Errorf("site 2 proxy_port = %v, want 8080", res2["proxy_port"])
	}
}
