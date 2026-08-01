// Package main runs the Master Router microservice for Enterprise Agent Platform.
package main

import (
	"context"
	"log"
	"os"
	"strings"

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

	args := os.Args[1:]
	if len(args) > 0 && args[0] == "web" {
		hasAPIServerAddr := false
		for _, arg := range args {
			if strings.HasPrefix(arg, "-api_server_address") {
				hasAPIServerAddr = true
				break
			}
		}
		if !hasAPIServerAddr {
			apiAddr := os.Getenv("API_SERVER_ADDRESS")
			if apiAddr == "" {
				apiAddr = "/api"
			}
			for i, arg := range args {
				if arg == "webui" {
					newArgs := make([]string, 0, len(args)+2)
					newArgs = append(newArgs, args[:i+1]...)
					newArgs = append(newArgs, "-api_server_address", apiAddr)
					newArgs = append(newArgs, args[i+1:]...)
					args = newArgs
					break
				}
			}
		}
	}

	l := full.NewLauncher()
	if err = l.Execute(ctx, config, args); err != nil {
		log.Fatalf("Run failed: %v\n\n%s", err, l.CommandLineSyntax())
	}
}
