// Package main runs the Master Router microservice for Enterprise Agent Platform.
package main

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"time"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/cmd/launcher"
	"google.golang.org/adk/v2/cmd/launcher/full"
	"google.golang.org/adk/v2/memory"
	"google.golang.org/adk/v2/model/gemini"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/telemetry"
	"google.golang.org/genai"

	"my-agent-enterprise/agents/chatagent"
	"my-agent-enterprise/internal/ratelimit"
)

//go:embed web/index.html
var webIndexHTML []byte

func getGCPProject() string {
	if proj := os.Getenv("GCP_PROJECT_ID"); proj != "" {
		return proj
	}
	return os.Getenv("GOOGLE_CLOUD_PROJECT")
}

func main() {
	ctx := context.Background()

	var clientCfg *genai.ClientConfig
	if apiKey := os.Getenv("GOOGLE_API_KEY"); apiKey != "" {
		clientCfg = &genai.ClientConfig{
			APIKey: apiKey,
		}
	} else {
		gcpProject := getGCPProject()
		if gcpProject != "" {
			clientCfg = &genai.ClientConfig{
				Backend:  genai.BackendVertexAI,
				Project:  gcpProject,
				Location: "us-central1",
			}
		}
	}

	model, err := gemini.NewModel(ctx, "gemini-flash-latest", clientCfg)
	if err != nil {
		log.Fatalf("failed to create gemini model: %v", err)
	}

	rootAgent, err := chatagent.New(model)
	if err != nil {
		log.Fatalf("failed to create chat agent: %v", err)
	}

	config := &launcher.Config{
		AgentLoader:    agent.NewSingleLoader(rootAgent),
		SessionService: session.InMemoryService(),
		MemoryService:  memory.InMemoryService(),
		TelemetryOptions: []telemetry.Option{
			telemetry.WithOtelToCloud(true),
		},
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Internal port for background ADK launcher API server
	internalAPIPort := "8081"

	// Start ADK Launcher API server on internal port with 10-minute timeouts for long agent runs
	go func() {
		l := full.NewLauncher()
		apiArgs := []string{
			"web",
			"-port", internalAPIPort,
			"-write-timeout", "10m",
			"-read-timeout", "10m",
			"-idle-timeout", "10m",
			"api",
		}
		if err := l.Execute(ctx, config, apiArgs); err != nil {
			log.Printf("ADK Launcher API server exited: %v", err)
		}
	}()

	time.Sleep(500 * time.Millisecond)

	apiTarget, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%s", internalAPIPort))
	apiProxy := httputil.NewSingleHostReverseProxy(apiTarget)
	apiProxy.Transport = &http.Transport{
		ResponseHeaderTimeout: 10 * time.Minute,
		IdleConnTimeout:       10 * time.Minute,
	}
	apiProxy.FlushInterval = 100 * time.Millisecond

	// Agent runs invoke Gemini and cost real money, so the public demo meters
	// them per visitor. Only the run endpoints are metered; the UI, session
	// bookkeeping, and app listing stay free so the page works after the quota
	// is spent (the simulated pipeline remains fully usable).
	limiter := ratelimit.New(ratelimit.Config{
		RunsPerMinute: envInt("DEMO_RUNS_PER_MINUTE", 2),
		RunsPerDay:    envInt("DEMO_RUNS_PER_DAY", 3),
		Paths:         []string{"/api/run"},
		// Only when Cloudflare fronts every request AND the origin is not
		// directly reachable; otherwise CF-Connecting-IP is forgeable.
		TrustCloudflare: os.Getenv("TRUST_CLOUDFLARE") == "true",
		// Injected by a Cloudflare Transform Rule; gates metered paths so the
		// bare origin URL cannot be used to bypass the quota.
		OriginSecret: os.Getenv("ORIGIN_SHARED_SECRET"),
	})

	mux := http.NewServeMux()

	// Remaining allowance, so the UI can show it before a run is attempted.
	mux.HandleFunc("/api/limits", limiter.StatusHandler)

	// Reverse proxy /api/ calls to ADK backend API server
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		apiProxy.ServeHTTP(w, r)
	})

	// Serve Production Command Center UI on root / and all non-API paths
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(webIndexHTML)
	})

	log.Printf("Enterprise Master Router listening on port %s (serving Production Command Center UI)", port)
	log.Printf("Demo quota: %d live runs/day, %d/min per client IP", envInt("DEMO_RUNS_PER_DAY", 3), envInt("DEMO_RUNS_PER_MINUTE", 2))
	if err := http.ListenAndServe(":"+port, limiter.Middleware(mux)); err != nil {
		log.Fatalf("HTTP server failed: %v", err)
	}
}

// envInt reads an integer env var, falling back to def when unset or invalid.
func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		log.Printf("invalid %s=%q, using default %d", key, v, def)
		return def
	}
	return n
}
