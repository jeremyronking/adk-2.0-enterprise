# ADK 2.0 Enterprise Agent Platform Reference

A reference implementation of a multi-agent system built with the **[Google Agent Development Kit (ADK) 2.0](https://google.github.io/adk-docs/) for Go**, with a live operations UI that visualises agent execution as it happens.

The demo scenario is a construction workflow: an orchestrator drives five build phases, a building inspector agent audits each one, and failed inspections trigger a repair-and-re-inspect loop — all decided by the model at runtime, not scripted.

![Command Center — a completed live run, showing the interior phase after two inspector-driven repair cycles](docs/command-center.jpg)

*A real Gemini-backed run. The interior phase failed inspection twice; each failure dispatched a repair and a re-inspection before passing.*

---

## What this demonstrates

- **Agent composition via tools.** Sub-agents are wrapped with `agenttool` and exposed to an orchestrator as callable tools, so the LLM decides the workflow order rather than a hardcoded state machine.
- **A self-correcting loop.** The inspector returns `PASS` or `FIX`. On `FIX`, the orchestrator dispatches a repair to the relevant trade agent and re-inspects — repeating until the phase passes.
- **Streaming execution telemetry.** The UI consumes ADK's SSE event stream and maps each `functionCall` / `functionResponse` onto stage cards, a per-phase thought stream, and an inspector audit drawer.
- **Compose-or-distribute.** The same agent packages run either embedded in one process or as independently deployable services.

---

## Architecture

The router process composes every agent **in-process** through a local registry. `chat_agent` is the root router; sub-agents register themselves via package `init()` and are discovered at startup.

```
 Browser ──HTTP──▶ router (cmd/router)
                    │
                    ├── serves the Command Center UI (embedded HTML)
                    ├── proxies /api/* ──▶ ADK REST server (in-process, port 8081)
                    └── meters live runs per client IP
                                              │
                                              ▼
                                     chat_agent  (root router)
                                              │  transfer_to_agent
                                              ▼
                                    construction_agent  (orchestrator)
                                              │  agenttool calls
              ┌───────────────┬───────────────┼───────────────┬───────────────┐
              ▼               ▼               ▼               ▼               ▼
      foundation_agent  framing_agent   roofing_agent  exterior_agent  interior_agent
                                              │
                                              ▼
                                      inspector_agent  ──▶ PASS | FIX
```

Because sub-agents are invoked as `agenttool` calls, they do **not** emit their own top-level events. Every event on the wire is authored by `construction_agent`; the active phase is identified by the *tool name*, not the event author. The UI relies on this when mapping events to stage cards.

### Independently deployable services

`cmd/construction`, `cmd/weather`, and `cmd/time` build the same agents as standalone ADK servers, each with its own container and Cloud Run target. They are deployable on their own; the router does not currently call them over the network — it embeds the agent packages directly.

| Binary | Entry point | Role |
| :--- | :--- | :--- |
| `router` | `cmd/router/main.go` | Command Center UI + embedded root agent |
| `construction` | `cmd/construction/main.go` | Construction orchestrator + trade/inspector agents |
| `weather` | `cmd/weather/main.go` | Weather forecast agent (Open-Meteo) |
| `time` | `cmd/time/main.go` | Time & timezone agent |

---

## Quick start

```bash
git clone https://github.com/<your-account>/adk-2.0-enterprise.git
cd adk-2.0-enterprise

cp .env.example .env
# Add a Gemini API key from https://aistudio.google.com/apikey
$EDITOR .env

set -a && . ./.env && set +a
go run ./cmd/router
```

Open <http://localhost:8080>.

Pick an **Execution Mode**:

| Mode | Behaviour | Cost |
| :--- | :--- | :--- |
| **Live ADK Backend Agent API** | Real Gemini-backed agent run over `/api/run_sse` | ~$0.06–0.08 per run |
| **Simulated pipeline** | Scripted walkthrough of the same UI | Free |

### Configuration

| Variable | Default | Purpose |
| :--- | :--- | :--- |
| `GOOGLE_API_KEY` | — | Gemini Developer API key. Takes precedence when set. |
| `GCP_PROJECT_ID` | — | Use Vertex AI via ADC instead of an API key. |
| `PORT` | `8080` | HTTP listen port. |
| `DEMO_RUNS_PER_DAY` | `3` | Live runs allowed per client IP per day. |
| `DEMO_RUNS_PER_MINUTE` | `2` | Burst limit per client IP. |

---

## Demo quota

Live runs invoke Gemini and cost real money, so a public deployment meters them per client IP: a daily cap plus a per-minute burst limit, keyed on the leftmost `X-Forwarded-For` entry (the real client behind Cloud Run's load balancer).

Only the agent-invoking endpoints are metered. The UI, session bookkeeping, and app listing stay free, so the page still works once the quota is spent — it falls back to the simulated pipeline and says so. `GET /api/limits` reports the caller's remaining allowance.

**The counter is in-process.** With more than one serving instance a visitor's effective allowance is multiplied by the instance count, so the router deploys with `--max-instances 1`. That is right for a demo and also hard-caps cost; a real deployment would move the counter to a shared store (Redis, Firestore) and drop the instance limit.

See [`internal/ratelimit/`](internal/ratelimit/).

---

## Streaming model

The UI streams from `/api/run_sse`. One detail is worth knowing, because it is easy to get wrong:

`/api/run_sse` **always** streams. The `streaming` flag in the request body controls *granularity*, not whether streaming happens.

| `streaming` | Emits | Events per run |
| :--- | :--- | ---: |
| `false` | One event per completed agent step | ~27 |
| `true` | One event per **token** (`partial: true`) | ~2,000 |

Partial events are never persisted to the session — they exist only to be painted as they arrive. That makes them invisible in the trace UI while still flooding a live client, so a UI that logs raw chunks appears to hang while the trace looks perfectly normal.

This UI renders discrete phases rather than streaming prose, so it sends `streaming: false` and gets complete `functionCall` / `functionResponse` events, which are what the stage-card mapping needs.

---

## Development

```bash
make check   # gofmt, go vet, go test
make test    # tests only
make build   # build all binaries into bin/
```

The Command Center UI lives in `web/index.html` and is embedded into the router binary with `go:embed`. Two byte-identical copies exist for the build and docs — keep all three in sync:

```
web/index.html                 # source of truth
cmd/router/web/index.html      # embedded into the binary
docs/command_center_demo.html  # standalone preview
```

---

## Deployment

Deployment targets Google Cloud Run. `PROJECT_ID` defaults to your active `gcloud` project:

```bash
gcloud config set project <your-project>

make deploy-router
make deploy-all
```

`deploy-*` reads `GOOGLE_API_KEY` from `.env` and passes it as a Cloud Run environment variable. `.env` is gitignored and is never baked into the image.

See [`docs/deployment.md`](docs/deployment.md), [`docs/architecture.md`](docs/architecture.md), and [`docs/coding_conventions.md`](docs/coding_conventions.md).

---

## Project layout

```
agents/           chatagent (root router), construction, weather, time
cmd/              one main package per deployable binary
internal/
  ratelimit/      per-IP daily + burst quota for the public demo
  registry/       in-process sub-agent registration and discovery
  openmeteo/      weather API client
web/              Command Center UI (embedded via go:embed)
docs/             architecture, deployment, conventions
```

---

## License

Reference implementation provided as-is for demonstration purposes.
