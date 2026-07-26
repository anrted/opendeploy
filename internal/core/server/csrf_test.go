package server

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"testing"

	csrf "filippo.io/csrf/gorilla"
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

func TestCSRFExemptRequest(t *testing.T) {
	tests := []struct {
		name          string
		path          string
		authorization string
		want          bool
	}{
		{name: "login", path: "/api/v1/auth/login", want: true},
		{name: "refresh", path: "/api/v1/auth/refresh", want: true},
		{name: "bearer API", path: "/api/v1/modules", authorization: "Bearer token", want: true},
		{name: "lowercase bearer", path: "/api/v1/modules", authorization: "bearer token", want: true},
		{name: "anonymous mutation", path: "/api/v1/modules", want: false},
		{name: "unrelated authorization", path: "/api/v1/modules", authorization: "Basic value", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, test.path, nil)
			request.Header.Set("Authorization", test.authorization)
			if got := csrfExemptRequest(request); got != test.want {
				t.Fatalf("csrfExemptRequest() = %t, want %t", got, test.want)
			}
		})
	}
}
