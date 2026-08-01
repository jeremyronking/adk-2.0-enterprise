package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProductionUIEndpoints(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(webIndexHTML)
	})

	endpoints := []string{"/", "/demo", "/command-center"}
	for _, ep := range endpoints {
		req := httptest.NewRequest(http.MethodGet, ep, nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected HTTP 200 for %s, got %d", ep, rec.Code)
		}

		body := rec.Body.String()
		if !strings.Contains(body, "Enterprise Agent Platform") {
			t.Errorf("expected HTML body for %s to contain platform title", ep)
		}
		if !strings.Contains(body, "Multi-Agent Construction Microservices") {
			t.Errorf("expected HTML body for %s to contain subtitle", ep)
		}
	}
}
