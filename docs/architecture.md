# Enterprise Microservice Architecture

This document describes the distributed multi-agent microservice architecture, component layout, and ADK 2.0 integration for the Enterprise Agent Platform reference implementation (`adk-2.0-enterprise`).

---

## 1. System Overview

The **Enterprise Agent Platform** is built using Google's **Agent Development Kit (ADK) 2.0** (`google.golang.org/adk/v2`) in Go. Agent packages are written so they can be composed two ways from the same source: embedded in a single process, or deployed as independent containerized services on Cloud Run.

**The router uses in-process composition.** Sub-agents register themselves with [`internal/registry`](../internal/registry) via package `init()`, and `chat_agent` discovers them at startup. There is no network hop between the router and its sub-agents.

```
 Browser ──HTTP──▶ router (cmd/router)
                    │
                    ├── serves the Command Center UI (go:embed)
                    ├── proxies /api/* ──▶ ADK REST server (in-process, port 8081)
                    └── per-IP demo quota (internal/ratelimit)
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

The `construction`, `weather`, and `time` binaries wrap the same agent packages as standalone ADK servers with their own Cloud Run targets. They are independently deployable, but the router does not currently call them over the network — it links the packages directly.

### Event authorship

Because sub-agents are invoked as `agenttool` calls rather than transferred to, they do **not** emit their own top-level events. Every event in a construction run is authored by `construction_agent`, appearing as `functionCall` / `functionResponse` pairs whose `name` is the sub-agent. Any consumer mapping events to UI state must key off the **tool name**, not the event author.

---

## 2. Microservice Inventory

| Service Name | Entry Point | Target Directory | Responsibility |
| :--- | :--- | :--- | :--- |
| **`adk-enterprise-router`** | [`cmd/router/main.go`](../cmd/router/main.go) | [`agents/chatagent`](../agents/chatagent) | Serves the Command Center UI (a single `go:embed`-ed HTML page) and hosts the root `chat_agent` with all sub-agents composed in-process. |
| **`adk-enterprise-construction`** | [`cmd/construction/main.go`](../cmd/construction/main.go) | [`agents/construction`](../agents/construction) | Construction simulation orchestrator managing specialized trade sub-agents (framing, electrical, plumbing, finishing). |
| **`adk-enterprise-weather`** | [`cmd/weather/main.go`](../cmd/weather/main.go) | [`agents/weatheragent`](../agents/weatheragent) | Standalone weather forecast service powered by Open-Meteo geocoding and forecast APIs. |
| **`adk-enterprise-time`** | [`cmd/time/main.go`](../cmd/time/main.go) | [`agents/timeagent`](../agents/timeagent) | Standalone date, time, and timezone resolution service with Go `tzdata`. |

---

## 3. Construction Trade Agent Hierarchy

Inside the `adk-enterprise-construction` microservice, the root construction agent orchestrates specialized trade sub-agents using a hierarchical sub-agent pattern:

```
                      ┌───────────────────────────────────┐
                      │    construction.New(model)        │
                      │  (Construction Orchestrator)      │
                      └─────────────────┬─────────────────┘
                                        │ (delegates via agenttool)
       ┌────────────────────────┬───────┴────────┬────────────────────────┐
       ▼                        ▼                ▼                        ▼
┌──────────────┐        ┌──────────────┐  ┌──────────────┐        ┌──────────────┐
┌ trade_framing│        │trade_electric│  │trade_plumbing│        │trade_finishin│
└──────────────┘        └──────────────┘  └──────────────┘        └──────────────┘
```

- Each trade agent is instantiated via `NewTradeAgent` in [`agents/construction/trades.go`](../agents/construction/trades.go).
- Each trade agent wraps a dedicated `NewTradeTool` handler that executes phase-specific simulation operations.

---

## 4. ADK 2.0 Infrastructure Integration

- **Launcher**: Every microservice uses `google.golang.org/adk/v2/cmd/launcher/full` for standard CLI (`console`) and HTTP server (`web`) startup routines.
- **Model**: `gemini.NewModel(ctx, "gemini-flash-latest", clientCfg)` with automatic fallback between GCP Vertex AI backend (`GCP_PROJECT_ID`) and Google AI Studio API key (`GOOGLE_API_KEY`).
- **Telemetry**: Distributed OpenTelemetry tracing via `telemetry.WithOtelToCloud(true)` exporting trace spans directly to GCP Cloud Trace and Cloud Logging when deployed on Cloud Run.
