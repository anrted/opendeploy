package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/anrted/opendeploy/internal/core/auth"
	"github.com/go-chi/chi/v5"
)

func TestModuleRouterAppliesReadAndMutationPermissions(t *testing.T) {
	router := chi.NewRouter()
	wrapper := chiRouterWrapper{
		Router:             router,
		prefix:             "/modules/test",
		readPermission:     auth.PermModuleView,
		mutationPermission: auth.PermModuleConfigure,
	}
	wrapper.Get("/status", func(w, _ any) { w.(http.ResponseWriter).WriteHeader(http.StatusNoContent) })
	wrapper.Post("/action", func(w, _ any) { w.(http.ResponseWriter).WriteHeader(http.StatusNoContent) })

	tests := []struct {
		name   string
		method string
		path   string
		role   auth.Role
		want   int
	}{
		{"viewer reads", http.MethodGet, "/modules/test/status", auth.RoleViewer, http.StatusNoContent},
		{"viewer cannot mutate", http.MethodPost, "/modules/test/action", auth.RoleViewer, http.StatusForbidden},
		{"operator mutates", http.MethodPost, "/modules/test/action", auth.RoleOperator, http.StatusNoContent},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, nil)
			request = request.WithContext(auth.WithPrincipal(request.Context(), &auth.Principal{Role: test.role}))
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != test.want {
				t.Fatalf("status = %d, want %d", response.Code, test.want)
			}
		})
	}
}
