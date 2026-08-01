---
name: enterprise-microservice-generator
description: Scaffolds a new standalone ADK 2.0 microservice for adk-2.0-enterprise, including cmd entrypoint, agent package, Makefile targets, and router integration.
---

# Enterprise Microservice Generator Skill

Use this skill when adding a new standalone ADK 2.0 microservice to the enterprise platform repository.

---

## Workflow Steps

### Step 1: Create the Agent Package
Create a new domain package directory `agents/<service_name>/` and file `agents/<service_name>/<service_name>.go`.

Export `Name` and `Description` constants and implement the `New(m model.LLM)` constructor:

```go
package <service_name>

import (
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/tool"
)

const Name = "<service_name>_agent"
const Description = "Handles ... for the enterprise platform."

func New(m model.LLM) (agent.Agent, error) {
	// Initialize tools here
	return llmagent.New(llmagent.Config{
		Name:        Name,
		Model:       m,
		Description: Description,
		Instruction: "You are the " + Name + ". Execute operations using tools and summarize output.",
		Tools:       []tool.Tool{},
	})
}
```

### Step 2: Create the Service Entrypoint (`cmd/<service_name>/main.go`)
Create directory `cmd/<service_name>/` and `cmd/<service_name>/main.go`:

```go
package main

import (
	"context"
	"log"
	"os"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/cmd/launcher"
	"google.golang.org/adk/v2/cmd/launcher/full"
	"google.golang.org/adk/v2/memory"
	"google.golang.org/adk/v2/model/gemini"
	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/telemetry"
	"google.golang.org/genai"

	"my-agent-enterprise/agents/<service_name>"
)

func main() {
	ctx := context.Background()
	clientCfg := &genai.ClientConfig{}
	if apiKey := os.Getenv("GOOGLE_API_KEY"); apiKey != "" {
		clientCfg.APIKey = apiKey
	}

	model, err := gemini.NewModel(ctx, "gemini-flash-latest", clientCfg)
	if err != nil {
		log.Fatalf("failed to create gemini model: %v", err)
	}

	svcAgent, err := <service_name>.New(model)
	if err != nil {
		log.Fatalf("failed to create agent: %v", err)
	}

	config := &launcher.Config{
		AgentLoader:    agent.NewSingleLoader(svcAgent),
		SessionService: session.InMemoryService(),
		MemoryService:  memory.InMemoryService(),
		TelemetryOptions: []telemetry.Option{
			telemetry.WithOtelToCloud(true),
		},
	}

	args := os.Args[1:]
	if len(args) == 0 {
		args = []string{"web", "-port", "8080", "api"}
	}

	l := full.NewLauncher()
	if err = l.Execute(ctx, config, args); err != nil {
		log.Fatalf("Run failed: %v", err)
	}
}
```

### Step 3: Add Makefile Targets
Add build and deploy targets to [`Makefile`](../../Makefile):

```makefile
build:
	...
	go build -o bin/<service_name> ./cmd/<service_name>

deploy-<service_name>:
	gcloud builds submit --project=$(PROJECT_ID) --config=cloudbuild.yaml --substitutions=_SERVICE=<service_name>,_IMAGE=gcr.io/$(PROJECT_ID)/adk-enterprise-<service_name>:latest .
	gcloud run deploy adk-enterprise-<service_name> \
		--project=$(PROJECT_ID) \
		--image gcr.io/$(PROJECT_ID)/adk-enterprise-<service_name>:latest \
		--platform managed \
		--region $(REGION) \
		--allow-unauthenticated \
		--set-env-vars GCP_PROJECT_ID="$(PROJECT_ID)",GOOGLE_API_KEY="$$(grep GOOGLE_API_KEY .env 2>/dev/null | cut -d'=' -f2 | tr -d '\"')" \
		--port 8080
```

### Step 4: Unit Testing & Verification
Create `agents/<service_name>/<service_name>_test.go` and run:
```bash
make check
```
