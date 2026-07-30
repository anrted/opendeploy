package siteruntime

import "testing"

func TestBelowRejectsTraversalAndRoot(t *testing.T) {
	for _, candidate := range []string{
		"/var/www",
		"/var/www/../etc",
		"/var/www-site/example",
		"var/www/example",
	} {
		if below(candidate, "/var/www") {
			t.Fatalf("below(%q) = true", candidate)
		}
	}
	if !below("/var/www/example/public", "/var/www") {
		t.Fatal("valid site path was rejected")
	}
}

func TestValidDomainRejectsPathLikeInput(t *testing.T) {
	for _, domain := range []string{"", "../example.com", "example..com", "example.com/config"} {
		if validDomain(domain) {
			t.Fatalf("validDomain(%q) = true", domain)
		}
	}
	if !validDomain("app.example.com") {
		t.Fatal("valid domain was rejected")
	}
}
