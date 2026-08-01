# Enterprise Agent Platform Makefile
#
# PROJECT_ID defaults to your active gcloud project. Override per-invocation
# with `make deploy-router PROJECT_ID=my-project`.
PROJECT_ID ?= $(shell gcloud config get-value project 2>/dev/null)
REGION ?= us-central1

.PHONY: all check test build deploy-router deploy-construction deploy-weather deploy-time deploy-all

all: check build

check:
	@echo "==> Formatting code..."
	gofmt -s -w .
	@echo "==> Running go vet..."
	go vet ./...
	@echo "==> Running unit tests..."
	go test -v ./...

test:
	go test -v ./...

build:
	go build -o bin/router ./cmd/router
	go build -o bin/construction ./cmd/construction
	go build -o bin/weather ./cmd/weather
	go build -o bin/time ./cmd/time

deploy-router:
	gcloud builds submit --project=$(PROJECT_ID) --config=cloudbuild.yaml --substitutions=_SERVICE=router,_IMAGE=gcr.io/$(PROJECT_ID)/adk-enterprise-router:latest .
	gcloud run deploy adk-enterprise-router \
		--project=$(PROJECT_ID) \
		--image gcr.io/$(PROJECT_ID)/adk-enterprise-router:latest \
		--platform managed \
		--region $(REGION) \
		--allow-unauthenticated \
		--set-env-vars GCP_PROJECT_ID="$(PROJECT_ID)",GOOGLE_API_KEY="$$(grep GOOGLE_API_KEY .env 2>/dev/null | cut -d'=' -f2 | tr -d '\"')" \
		--port 8080

deploy-construction:
	gcloud builds submit --project=$(PROJECT_ID) --config=cloudbuild.yaml --substitutions=_SERVICE=construction,_IMAGE=gcr.io/$(PROJECT_ID)/adk-enterprise-construction:latest .
	gcloud run deploy adk-enterprise-construction \
		--project=$(PROJECT_ID) \
		--image gcr.io/$(PROJECT_ID)/adk-enterprise-construction:latest \
		--platform managed \
		--region $(REGION) \
		--allow-unauthenticated \
		--set-env-vars GCP_PROJECT_ID="$(PROJECT_ID)",GOOGLE_API_KEY="$$(grep GOOGLE_API_KEY .env 2>/dev/null | cut -d'=' -f2 | tr -d '\"')" \
		--port 8080

deploy-weather:
	gcloud builds submit --project=$(PROJECT_ID) --config=cloudbuild.yaml --substitutions=_SERVICE=weather,_IMAGE=gcr.io/$(PROJECT_ID)/adk-enterprise-weather:latest .
	gcloud run deploy adk-enterprise-weather \
		--project=$(PROJECT_ID) \
		--image gcr.io/$(PROJECT_ID)/adk-enterprise-weather:latest \
		--platform managed \
		--region $(REGION) \
		--allow-unauthenticated \
		--set-env-vars GCP_PROJECT_ID="$(PROJECT_ID)",GOOGLE_API_KEY="$$(grep GOOGLE_API_KEY .env 2>/dev/null | cut -d'=' -f2 | tr -d '\"')" \
		--port 8080

deploy-time:
	gcloud builds submit --project=$(PROJECT_ID) --config=cloudbuild.yaml --substitutions=_SERVICE=time,_IMAGE=gcr.io/$(PROJECT_ID)/adk-enterprise-time:latest .
	gcloud run deploy adk-enterprise-time \
		--project=$(PROJECT_ID) \
		--image gcr.io/$(PROJECT_ID)/adk-enterprise-time:latest \
		--platform managed \
		--region $(REGION) \
		--allow-unauthenticated \
		--set-env-vars GCP_PROJECT_ID="$(PROJECT_ID)",GOOGLE_API_KEY="$$(grep GOOGLE_API_KEY .env 2>/dev/null | cut -d'=' -f2 | tr -d '\"')" \
		--port 8080

deploy-all: deploy-router deploy-construction deploy-weather deploy-time
