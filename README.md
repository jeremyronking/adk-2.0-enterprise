# ADK 2.0 Enterprise Agent Platform Reference

Enterprise reference implementation demonstrating a distributed multi-agent microservice architecture built with **Google Agent Development Kit (ADK 2.0)** in Go.

---

## 🏗 Microservice Architecture

```
                                    ┌─────────────────────────────┐
                                    │    adk-enterprise-router    │
                                    │  (Master Router Web UI)     │
                                    └──────────────┬──────────────┘
                                                   │ A2A Protocol
                   ┌───────────────────────────────┼───────────────────────────────┐
                   ▼                               ▼                               ▼
    ┌─────────────────────────────┐ ┌─────────────────────────────┐ ┌─────────────────────────────┐
    │ adk-enterprise-construction │ │    adk-enterprise-weather   │ │      adk-enterprise-time    │
    │ (Construction Orchestrator) │ │       (Weather Agent)       │ │        (Time Agent)         │
    └─────────────────────────────┘ └─────────────────────────────┘ └─────────────────────────────┘
```

---

## 🚀 Microservices

| Service Name | Entry Point | Cloud Run Endpoint | Description |
| :--- | :--- | :--- | :--- |
| **`adk-enterprise-router`** | `cmd/router/main.go` | `https://adk-enterprise-router-153583612125.us-central1.run.app` | Master router serving Angular Web UI and A2A routing. |
| **`adk-enterprise-construction`** | `cmd/construction/main.go` | `https://adk-enterprise-construction-153583612125.us-central1.run.app` | Construction simulation orchestrator & trade agents. |
| **`adk-enterprise-weather`** | `cmd/weather/main.go` | `https://adk-enterprise-weather-153583612125.us-central1.run.app` | Weather forecast sub-agent microservice. |
| **`adk-enterprise-time`** | `cmd/time/main.go` | `https://adk-enterprise-time-153583612125.us-central1.run.app` | Time & timezone sub-agent microservice. |

---

## 🛠 Local Commands

```bash
# Run unit tests across all microservices
make test

# Full quality gate
make check

# Deploy all microservices to GCP Cloud Run
make deploy-all
```
