# Enterprise Microservice Architecture

This document describes the distributed multi-agent microservice architecture, component layout, and ADK 2.0 integration for the Enterprise Agent Platform reference implementation (`adk-2.0-enterprise`).

---

## 1. System Overview

The **Enterprise Agent Platform** is built using Google's **Agent Development Kit (ADK) 2.0** (`google.golang.org/adk/v2`) in Go. Unlike monolithic single-binary agent setups, the enterprise architecture splits orchestrators, domain services, and trade simulation agents into independent, containerized HTTP microservices deployed to GCP Cloud Run.

```
                                    ┌─────────────────────────────┐
                                    │    adk-enterprise-router    │
                                    │  (Master Router & Web UI)   │
                                    └──────────────┬──────────────┘
                                                   │ A2A Protocol / REST
                   ┌───────────────────────────────┼───────────────────────────────┐
                   ▼                               ▼                               ▼
    ┌─────────────────────────────┐ ┌─────────────────────────────┐ ┌─────────────────────────────┐
    │ adk-enterprise-construction │ │    adk-enterprise-weather   │ │      adk-enterprise-time    │
    │ (Construction Orchestrator) │ │       (Weather Agent)       │ │        (Time Agent)         │
    └─────────────────────────────┘ └─────────────────────────────┘ └─────────────────────────────┘
```

---

## 2. Microservice Inventory

| Service Name | Entry Point | Target Directory | Responsibility |
| :--- | :--- | :--- | :--- |
| **`adk-enterprise-router`** | [`cmd/router/main.go`](../cmd/router/main.go) | [`agents/chatagent`](../agents/chatagent) | Master router serving Angular Web UI, delegating requests across microservices. |
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
