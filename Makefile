# Common Variables
APP_VERSION ?= $(shell git describe --tags --always --dirty) # unavailable in Docker unless we copy `.git`
ifeq ($(strip $(APP_VERSION)),)
# works in Docker after running 'make write-build-info'
# equired to run 'go build' with properly populated ldflags
	APP_VERSION := $(file < VERSION)
endif

COMMIT ?= $(shell git rev-parse --short HEAD)
ifeq ($(strip $(COMMIT)),)
	COMMIT := $(file < COMMIT)
endif

BRANCH ?= $(shell git rev-parse --abbrev-ref HEAD)
ifeq ($(strip $(BRANCH)),)
	BRANCH := $(file < BRANCH)
endif

USER=$(shell whoami)
BINARY=go-svc-template

# Go Variables
PKG=github.com/gesundheitscloud/go-svc-template/pkg/config
LDFLAGS="-X '$(PKG).Version=$(APP_VERSION)' -X '$(PKG).Commit=$(COMMIT)' -X '$(PKG).Branch=$(BRANCH)' -X '$(PKG).BuildUser=$(USER)'"
GOCMD=go
GOBUILD=$(GOCMD) build -ldflags $(LDFLAGS)
GOTEST=$(GOCMD) test
SRC = cmd/api/*.go
DUMMY_SECRET=very-secure-secret
define LOCAL_VARIABLES
GO_SVC_TEMPLATE_SERVICE_SECRET=$(DUMMY_SECRET) \
GO_SVC_TEMPLATE_HUMAN_READABLE_LOGS=true \
GO_SVC_TEMPLATE_DB_SSL_MODE=disable
endef

# Docker Variables
DOCKER_REGISTRY ?= crsensorhub.azurecr.io
DOCKER_IMAGE=$(DOCKER_REGISTRY)/$(BINARY)
CONTAINER_NAME=$(BINARY)
PORT=9000
DB_IMAGE=postgres
DB_CONTAINER_NAME=$(BINARY)-postgres
DB_PORT=6000

# Deploy variables
APP ?= $(BINARY)
ENVIRONMENT ?= local
KUBECONFIG ?= $(HOME)/.kube/config
VALUES_YAML ?= "$(shell pwd)/deploy/$(ENVIRONMENT)/values.yaml"
SECRETS_YAML ?= "$(shell pwd)/deploy/local/secrets.yaml"
NAMESPACE ?= default

## ----------------------------------------------------------------------
## Help: Makefile for app: go-svc-template
## ----------------------------------------------------------------------

.PHONY: help
help:               ## Show this help (default)
	@grep -E '^([[[:alpha:]][[:graph:]]*[[:space:]]*)+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-22s\033[0m %s\n", $$1, $$2}'

.PHONY: version
version:              ## Display current version
	@echo $(APP_VERSION)

## ----------------------------------------------------------------------
## Github Actions (github workflow relies on those)
## ----------------------------------------------------------------------

.PHONY: test-gh-action
test-gh-action: ## Run tests natively in verbose mode
	$(LOCAL_VARIABLES) \
	$(GOTEST) -timeout 300s -cover -covermode=atomic -v ./... 2>&1 | tee test-result.out

.PHONY: docker-build db
docker-build db: write-build-info    ## Build Docker image
	docker buildx build \
		--cache-to type=gha,mode=max \
		--cache-from type=gha \
		--build-arg GITHUB_USER_TOKEN \
		-t $(DOCKER_IMAGE):$(APP_VERSION) \
		-f build/Dockerfile \
		--target run \
		--load \
		.

.PHONY: write-build-info
write-build-info: ## Persist build parameters that require git
	@echo "$(APP_VERSION)" > VERSION
	@echo "$(COMMIT)" > COMMIT
	@echo "$(BRANCH)" > BRANCH

.PHONY: docker-push
docker-push:        ## Push Docker image to registry
	docker push $(DOCKER_IMAGE):$(APP_VERSION)

.PHONY: deploy
deploy: helm-deploy reload     ## Redeploy and reload secrets

.PHONY: reload
reload:   ## Reloads configuration - currently: gracefully restarts K8s deployment
	kubectl -n $(NAMESPACE) rollout restart deployment.apps/$(APP)

.PHONY: helm-deploy
helm-deploy:   ## Deploy to kubernetes using Helm
	sed -e 's/<github_release>/$(APP_VERSION)/g' deploy/helm-chart/Chart.versionless.yaml > deploy/helm-chart/Chart.yaml
	helm upgrade --install $(APP) \
		-f $(VALUES_YAML) \
		-f $(SECRETS_YAML) \
		--namespace $(NAMESPACE) \
		--set imageTag=$(DOCKER_IMAGE):$(APP_VERSION) \
		--wait \
		deploy/helm-chart

## ----------------------------------------------------------------------
## Local Convenience
## ----------------------------------------------------------------------

.PHONY: clean
clean:              ## Remove compiled binary, versioned chart and docker containers
	rm -f $(BINARY)
	rm -f deploy/helm-chart/Chart.yaml
	-docker rm -f $(CONTAINER_NAME)
	-docker rm -f $(DB_CONTAINER_NAME)

.env: Makefile      ## Generate .env file from LOCAL_VARIABLES
	@echo '${LOCAL_VARIABLES}' | sed -E -e 's/([^=]*)=("[^"]*"|[^ ]*)[ ]*/\1=\2\n/g' -e 's/"//g' > $@

.docker.env: .env   ## Generate .docker.env file to be used for docker-run
	sed -E 's#$(shell pwd)/test-config#/etc/shared-config#g' $? > $@

.PHONY: local-test lt
local-test lt: ## Run tests natively
	$(LOCAL_VARIABLES) \
	$(GOTEST) -timeout 45s -cover -covermode=atomic ./...

.PHONY: test
test: lint unit-test-postgres  ## Run all test activities sequentially

.PHONY: unit-test-postgres
unit-test-postgres: docker-database local-test docker-database-delete         ## Run unit tests inside the Docker and uses Postgres DB

.PHONY: lint
lint:
	docker buildx build \
		--build-arg GITHUB_USER_TOKEN \
		-t "$(DOCKER_IMAGE):lint" \
		-f build/Dockerfile \
		--target lint \
		.

.PHONY: docker-database
docker-database: docker-database-delete ## Run database in Docker
	docker run --name $(DB_CONTAINER_NAME) -d \
		-e POSTGRES_DB=go-svc-template \
		-e POSTGRES_USER=go-svc-template \
		-e POSTGRES_PASSWORD=postgres \
		-p $(DB_PORT):5432 $(DB_IMAGE)
	@until docker container exec -t $(DB_CONTAINER_NAME) pg_isready; do \
		>&2 echo "Postgres is unavailable - waiting for it... 😴"; \
		sleep 1; \
	done

.PHONY: docker-database-delete
docker-database-delete: ## Delete database in Docker
	-docker rm -f $(DB_CONTAINER_NAME)

.PHONY: build
build: ## Build app
	$(GOBUILD) -o $(BINARY) $(SRC)

.PHONY: run
run: ## Run app natively
	$(LOCAL_VARIABLES) \
	$(GOCMD) run $(SRC)

.PHONY: docker-run dr
docker-run dr: .docker.env ## Run app in Docker. Configure connection to a DB using GO_SVC_TEMPLATE_DB_HOST and GO_SVC_TEMPLATE_DB_PORT
	docker run --name $(DB_CONTAINER_NAME) -d \
		-e POSTGRES_DB=go-svc-template \
		-e POSTGRES_USER=go-svc-template \
		-e POSTGRES_PASSWORD=postgres \
		-p $(DB_PORT):5432 $(DB_IMAGE)
	docker run --name $(CONTAINER_NAME) -t -d \
		-e GO_SVC_TEMPLATE_DB_HOST=host.docker.internal \
		--env-file .docker.env \
		--mount type=bind,source=$$(pwd)/test-config,target=/etc/shared-config \
		-p $(PORT):$(PORT) \
		$(DOCKER_IMAGE):$(APP_VERSION)

.PHONY: local-install li
local-install li: shared-config  ## Deploy to local Kubernetes (check kubectl context beforehand)
	kubectl config use-context docker-desktop
	make deploy

.PHONY: local-delete ld
local-delete ld:    ## Delete local helm deployment
	kubectl config use-context docker-desktop
	helm delete $(APP) --namespace ${NAMESPACE}

.PHONY: docker-prune
docker-prune:       ## Delete local docker images of this repo except for the current version
	docker images | grep $(DOCKER_IMAGE) | grep -v $(APP_VERSION) | awk '{print $$3}' | xargs docker rmi -f
	docker ps --quiet --all --filter status=exited | xargs docker rm
