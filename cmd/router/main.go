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

	// Start ADK Launcher API server on internal port
	go func() {
		l := full.NewLauncher()
		apiArgs := []string{"web", "-port", internalAPIPort, "api"}
		if err := l.Execute(ctx, config, apiArgs); err != nil {
			log.Printf("ADK Launcher API server exited: %v", err)
		}
	}()

	time.Sleep(300 * time.Millisecond)

	apiTarget, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%s", internalAPIPort))
	apiProxy := httputil.NewSingleHostReverseProxy(apiTarget)

	mux := http.NewServeMux()

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
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("HTTP server failed: %v", err)
	}
}
