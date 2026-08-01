package openmeteo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestPlaceLabel(t *testing.T) {
	tests := []struct {
		name     string
		place    Place
		expected string
	}{
		{
			name: "full place",
			place: Place{
				Name:    "Springfield",
				Admin1:  "Illinois",
				Country: "United States",
			},
			expected: "Springfield, Illinois, United States",
		},
		{
			name: "matching name and admin1",
			place: Place{
				Name:    "Tokyo",
				Admin1:  "Tokyo",
				Country: "Japan",
			},
			expected: "Tokyo, Japan",
		},
		{
			name: "name and country only",
			place: Place{
				Name:    "Paris",
				Country: "France",
			},
			expected: "Paris, France",
		},
		{
			name: "name only",
			place: Place{
				Name: "Singapore",
			},
			expected: "Singapore",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.place.Label()
			if got != tt.expected {
				t.Errorf("Label() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestDescribeCode(t *testing.T) {
	tests := []struct {
		code     int
		expected string
	}{
		{0, "Clear sky"},
		{61, "Slight rain"},
		{95, "Thunderstorm"},
		{999, "Unknown conditions (WMO code 999)"},
	}

	for _, tt := range tests {
		got := DescribeCode(tt.code)
		if got != tt.expected {
			t.Errorf("DescribeCode(%d) = %q, want %q", tt.code, got, tt.expected)
		}
	}
}

func mockHTTPClient(server *httptest.Server) func() {
	origClient := httpClient
	httpClient = server.Client()
	return func() {
		httpClient = origClient
	}
}

func TestGeocode(t *testing.T) {
	t.Run("empty location", func(t *testing.T) {
		_, err := Geocode(context.Background(), "   ")
		if err == nil {
			t.Fatal("expected error for empty location, got nil")
		}
	})

	t.Run("successful geocode", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"results": [
					{
						"name": "Paris",
						"latitude": 48.8566,
						"longitude": 2.3522,
						"country": "France",
						"admin1": "Île-de-France",
						"timezone": "Europe/Paris"
					}
				]
			}`))
		}))
		defer server.Close()

		cleanup := mockHTTPClient(server)
		defer cleanup()

		// Temporarily replace geocodeURL via custom transport or direct test
		// In openmeteo.go geocodeURL is a const, so we test getJSON with the mock server directly
		// or verify mock responses using a custom RoundTripper.
	})
}

func TestGetJSONWithMockTransport(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("name") == "UnknownPlace" {
			_, _ = w.Write([]byte(`{"results": []}`))
			return
		}
		if r.URL.Query().Get("name") == "ErrorPlace" {
			http.Error(w, "server error", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(`{
			"results": [
				{
					"name": "London",
					"latitude": 51.5074,
					"longitude": -0.1278,
					"country": "United Kingdom",
					"timezone": "Europe/London"
				}
			]
		}`))
	}))
	defer server.Close()

	// Redirect transport to server
	origTransport := http.DefaultTransport
	httpClient = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			targetURL, _ := url.Parse(server.URL + req.URL.Path + "?" + req.URL.RawQuery)
			req.URL = targetURL
			return origTransport.RoundTrip(req)
		}),
	}
	defer func() { httpClient = &http.Client{Timeout: 10 * http.DefaultClient.Timeout} }()

	t.Run("valid location", func(t *testing.T) {
		p, err := Geocode(context.Background(), "London")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.Name != "London" || p.Timezone != "Europe/London" {
			t.Errorf("unexpected place: %+v", p)
		}
	})

	t.Run("place not found", func(t *testing.T) {
		_, err := Geocode(context.Background(), "UnknownPlace")
		if err == nil {
			t.Fatal("expected ErrPlaceNotFound, got nil")
		}
		if _, ok := err.(ErrPlaceNotFound); !ok {
			t.Errorf("expected ErrPlaceNotFound, got %T: %v", err, err)
		}
	})

	t.Run("server error", func(t *testing.T) {
		_, err := Geocode(context.Background(), "ErrorPlace")
		if err == nil {
			t.Fatal("expected HTTP error, got nil")
		}
	})
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
