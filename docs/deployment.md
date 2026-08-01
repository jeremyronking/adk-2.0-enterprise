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
| `DEMO_RUNS_PER_DAY` | Live agent runs allowed per client IP per day. | Optional | `3` |
| `DEMO_RUNS_PER_MINUTE` | Burst limit per client IP. | Optional | `2` |
| `TRUST_CLOUDFLARE` | Read the client IP from `CF-Connecting-IP`. Requires `ORIGIN_SHARED_SECRET`. | Optional | `false` |
| `ORIGIN_SHARED_SECRET` | Secret injected by the edge; metered endpoints reject requests without it. | Optional | unset |

---

## 5. Custom Domain via Cloudflare

The public demo is served from a subdomain fronted by Cloudflare. The ordering
matters: Google must reach the hostname directly to issue its managed
certificate, which the Cloudflare proxy would otherwise block.

**1. Verify the domain** for the GCP account (once per domain):

```bash
gcloud domains list-user-verified --project=$PROJECT_ID
```

**2. Create the Cloud Run domain mapping.** This is also what makes Cloud Run
accept the custom hostname in the `Host` header:

```bash
gcloud beta run domain-mappings create \
  --service=adk-enterprise-router \
  --domain=adk.example.com \
  --region=us-central1
```

**3. Add a CNAME to `ghs.googlehosted.com`, DNS-only (grey cloud).** Wait for
the certificate — Cloud Run reports `CertificateProvisioned` before Google's
edge actually serves it, so poll the endpoint rather than the status field:

```bash
until curl -sf -o /dev/null https://adk.example.com/api/limits; do sleep 30; done
```

**4. Add a Transform Rule** (Rules → Transform Rules → Modify Request Header)
matching `http.host eq "adk.example.com"`, setting `X-Origin-Auth` to a
generated secret. This is what lets the origin distinguish proxied traffic from
someone hitting the `run.app` URL directly.

**5. Switch the DNS record to proxied (orange cloud)** and set the Cloudflare
SSL mode to **Full (strict)**. Cloudflare connects to `ghs.googlehosted.com`
with SNI for the custom hostname, which the Google-managed certificate matches.

**6. Enable edge-aware client identification** on Cloud Run — only after the
Transform Rule is live, or metered endpoints will reject every request:

```bash
gcloud run services update adk-enterprise-router \
  --region=us-central1 \
  --update-env-vars TRUST_CLOUDFLARE=true,ORIGIN_SHARED_SECRET=<secret>
```

### Why both variables

Behind a proxy every request arrives from the proxy's address, so per-IP
accounting has to read `CF-Connecting-IP`. That header is only trustworthy if
the origin cannot be reached directly — otherwise anyone who finds the
`run.app` URL can forge it. The shared secret closes that path. Neither
variable is safe alone.

---

## 6. Security & Authentication

- **Cloud Run IAM**: Microservices run on Cloud Run with `--allow-unauthenticated` for public web access or can be configured with GCP IAM service-to-service authentication headers.
- **Vertex AI Fallback**: When `GOOGLE_API_KEY` is omitted, `genai.ClientConfig` automatically configures `genai.BackendVertexAI` using Application Default Credentials (ADC) in GCP `us-central1`.
