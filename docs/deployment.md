# Microservice Deployment & Cloud Run Operations

This document details containerization, GCP Cloud Build submission, Cloud Run deployment, and environment variable standards for `adk-2.0-enterprise`.

---

## 1. Deployment Architecture

Each microservice in the enterprise platform is built into a lightweight Linux container containerized via Docker and deployed to GCP Cloud Run.

```
                    ┌──────────────────────────────┐
                    │      GCP Cloud Build         │
                    │   (cloudbuild.yaml build)    │
                    └──────────────┬───────────────┘
                                   │ Image Push
                    ┌──────────────▼───────────────┐
                    │  Google Container Registry   │
                    │  (gcr.io/$PROJECT/$SERVICE)  │
                    └──────────────┬───────────────┘
                                   │ Deploy
                    ┌──────────────▼───────────────┐
                    │       GCP Cloud Run          │
                    │  (Managed Serverless HTTP)   │
                    └──────────────────────────────┘
```

---

## 2. Makefile Deployment Commands

The project [Makefile](../Makefile) provides targets for building and deploying each microservice individually or all at once:

```bash
# Deploy Master Router Microservice (Web UI + REST API)
make deploy-router PROJECT_ID=my-gcp-project REGION=us-central1

# Deploy Construction Orchestrator & Trade Microservices
make deploy-construction PROJECT_ID=my-gcp-project REGION=us-central1

# Deploy Standalone Weather Microservice
make deploy-weather PROJECT_ID=my-gcp-project REGION=us-central1

# Deploy Standalone Time Microservice
make deploy-time PROJECT_ID=my-gcp-project REGION=us-central1

# Deploy ALL Microservices sequentially
make deploy-all PROJECT_ID=my-gcp-project REGION=us-central1
```

---

## 3. GCP Cloud Build (`cloudbuild.yaml`)

Container builds are processed remotely using GCP Cloud Build. The build process accepts two substitution variables:
- `_SERVICE`: Target binary name in `cmd/` (e.g., `router`, `construction`, `weather`, `time`).
- `_IMAGE`: Fully-qualified container registry path (e.g., `gcr.io/my-project/adk-enterprise-router:latest`).

---

## 4. Environment Variable Configuration

| Variable | Description | Required | Default / Fallback |
| :--- | :--- | :--- | :--- |
| `GCP_PROJECT_ID` | Google Cloud project ID for Vertex AI models and OpenTelemetry trace exports. | Recommended | `GOOGLE_CLOUD_PROJECT` |
| `GOOGLE_API_KEY` | Gemini API key (Google AI Studio). If provided, model calls route to AI Studio. | Optional | Vertex AI ADC authentication if blank |
| `API_SERVER_ADDRESS` | Public endpoint or path prefix for REST API proxying in Web UI mode. | Optional | `/api` |
| `PORT` | Container HTTP listen port. | Optional | `8080` |

---

## 5. Security & Authentication

- **Cloud Run IAM**: Microservices run on Cloud Run with `--allow-unauthenticated` for public web access or can be configured with GCP IAM service-to-service authentication headers.
- **Vertex AI Fallback**: When `GOOGLE_API_KEY` is omitted, `genai.ClientConfig` automatically configures `genai.BackendVertexAI` using Application Default Credentials (ADC) in GCP `us-central1`.
