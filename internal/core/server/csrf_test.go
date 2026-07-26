package server

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/csrf"
)

func TestPlaintextCSRFHandshake(t *testing.T) {
	const header = "X-CSRF-Token"
	protect := csrf.Protect(
		[]byte("0123456789abcdef0123456789abcdef"),
		csrf.Secure(false),
		csrf.RequestHeader(header),
	)
	handler := protect(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Header().Set(header, csrf.Token(r))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	plaintextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.ServeHTTP(w, csrf.PlaintextHTTPRequest(r))
	})
	server := httptest.NewServer(plaintextHandler)
	defer server.Close()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("create cookie jar: %v", err)
	}
	client := &http.Client{Jar: jar}
	response, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("get CSRF token: %v", err)
	}
	token := response.Header.Get(header)
	_ = response.Body.Close()
	if token == "" {
		t.Fatal("CSRF response header is empty")
	}

	request, err := http.NewRequest(http.MethodPost, server.URL, nil)
	if err != nil {
		t.Fatalf("create POST request: %v", err)
	}
	request.Header.Set(header, token)
	response, err = client.Do(request)
	if err != nil {
		t.Fatalf("submit protected request: %v", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()
	if response.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("protected POST returned %d: %s", response.StatusCode, body)
	}
}
