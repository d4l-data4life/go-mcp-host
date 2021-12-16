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
CILINT_VERSION := v1.42
GO_VERSION := 1.17

PKG=github.com/gesundheitscloud/go-svc-template/pkg/config
LDFLAGS="-X '$(PKG).Version=$(APP_VERSION)' -X '$(PKG).Commit=$(COMMIT)' -X '$(PKG).Branch=$(BRANCH)' -X '$(PKG).BuildUser=$(USER)'"
GOCMD=go
GOBUILD=$(GOCMD) build -ldflags $(LDFLAGS)
GOTEST=$(GOCMD) test
PUBLIC_KEY_PATH="$(shell pwd)/test-keys/jwtpublickey.pem"
SRC = cmd/api/*.go
DUMMY_SECRET=very-secure-secret
define LOCAL_VARIABLES
GO_SVC_TEMPLATE_VEGA_JWT_PUBLIC_KEY_PATH=$(PUBLIC_KEY_PATH) \
GO_SVC_TEMPLATE_SERVICE_SECRET=$(DUMMY_SECRET) \
GO_SVC_TEMPLATE_HUMAN_READABLE_LOGS=true \
GO_SVC_TEMPLATE_DB_SSL_MODE=disable
endef

# Docker Variables
DOCKER_REGISTRY ?= phdp-snapshots.hpsgc.de
DOCKER_IMAGE=$(DOCKER_REGISTRY)/$(BINARY)
CONTAINER_NAME=$(BINARY)
PORT=9000
DB_IMAGE=postgres
DB_CONTAINER_NAME=$(BINARY)-postgres
DB_PORT=6000

# Deploy variables
APP ?= $(BINARY)
KUBECONFIG ?= $(HOME)/.kube/config
VALUES_YAML ?= "$(shell pwd)/deploy/local/values.yaml"
SECRETS_YAML ?= "$(shell pwd)/deploy/local/secrets.yaml"
NAMESPACE ?= default

## ----------------------------------------------------------------------
## Help: Makefile for app: go-svc-template
## ----------------------------------------------------------------------

.PHONY: help
help:               ## Show this help (default)
	@grep -E '^([[:graph:]]+[[:space:]]*)+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-22s\033[0m %s\n", $$1, $$2}'

.PHONY: version
version:              ## Display current version
	@echo $(APP_VERSION)

## ----------------------------------------------------------------------
## Mandatory for PHDP Jenkins
## ----------------------------------------------------------------------

.PHONY: write-build-info
write-build-info: ## Persist build parameters that require git
	@echo "$(APP_VERSION)" > VERSION
	@echo "$(COMMIT)" > COMMIT
	@echo "$(BRANCH)" > BRANCH

.PHONY: parallel-test
parallel-test:               ## Specifies space-separated list of targets that can be run in parallel
	@echo "lint unit-test-postgres"

.PHONY: clean
clean:              ## Remove compiled binary, versioned chart and docker containers
	rm -f $(BINARY)
	rm -f deploy/helm-chart/Chart.yaml
	-docker rm -f $(CONTAINER_NAME)
	-docker rm -f $(DB_CONTAINER_NAME)

.PHONY: test
test: lint unit-test-postgres  ## Run all test activities sequentially

.PHONY: unit-test-postgres
unit-test-postgres: docker-database unit-test docker-database-delete         ## Run unit tests inside the Docker and uses Postgres DB

.PHONY: unit-test
unit-test:          ## Run unit tests inside the Docker image
	DOCKER_BUILDKIT=1 \
	docker build \
		--build-arg GO_VERSION=$(GO_VERSION) \
		--build-arg CILINT_VERSION=${CILINT_VERSION} \
		--build-arg GITHUB_USER_TOKEN \
		--build-arg GO_SVC_TEMPLATE_DB_PORT=$(DB_PORT) \
		--build-arg GO_SVC_TEMPLATE_DB_HOST=172.17.0.1 \
		--build-arg GO_SVC_TEMPLATE_DB_SSL_MODE=disable \
		-t $(DOCKER_IMAGE):test \
		-f build/Dockerfile \
		--target unit-test \
		.

.PHONY: lint
lint:
	DOCKER_BUILDKIT=1 \
	docker build \
		--build-arg GO_VERSION=$(GO_VERSION) \
		--build-arg CILINT_VERSION=${CILINT_VERSION} \
		--build-arg GITHUB_USER_TOKEN \
		-t "$(DOCKER_IMAGE):lint" \
		-f build/Dockerfile \
		--target lint \
		.

.PHONY: docker-build db
docker-build db: write-build-info    ## Build Docker image
	DOCKER_BUILDKIT=1 \
	docker build \
		--build-arg GO_VERSION=$(GO_VERSION) \
		--build-arg GITHUB_USER_TOKEN \
		-t $(DOCKER_IMAGE):$(APP_VERSION) \
		-f build/Dockerfile \
		--target run \
		.

.PHONY: docker-push
docker-push:        ## Push Docker image to registry
	docker push $(DOCKER_IMAGE):$(APP_VERSION)

.PHONY: scan-docker-images
scan-docker-images:
	@echo "$(DOCKER_IMAGE):$(APP_VERSION)"

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
## Additional
## ----------------------------------------------------------------------

.PHONY: local-test lt
local-test lt: ## Run tests natively
	$(LOCAL_VARIABLES) \
	$(GOTEST) -timeout 45s -cover -covermode=atomic ./...

.PHONY: test-verbose
test-verbose: ## Run tests natively in verbose mode
	$(LOCAL_VARIABLES) \
	$(GOTEST) -timeout 45s -cover -covermode=atomic -v ./...

.PHONY: vendor
vendor: ## Download and tidy go depenencies
	@go mod tidy

.PHONY: build
build: ## Build app
	$(GOBUILD) -o $(BINARY) $(SRC)

.PHONY: run
run: config-download ## Run app natively
	$(LOCAL_VARIABLES) \
	$(GOCMD) run $(SRC)

.PHONY: docker-database
docker-database: ## Run database in Docker
	docker run --name $(DB_CONTAINER_NAME) -d \
		-e POSTGRES_DB=go-svc-template \
		-e POSTGRES_PASSWORD=postgres \
		-p $(DB_PORT):5432 $(DB_IMAGE)
	@until docker container exec -t $(DB_CONTAINER_NAME) pg_isready; do \
		>&2 echo "Postgres is unavailable - waiting for it... 😴"; \
		sleep 1; \
	done

.PHONY: docker-database-delete
docker-database-delete: ## Delete database in Docker
	-docker rm -f $(DB_CONTAINER_NAME)

.PHONY: docker-run dr
docker-run dr: config-download ## Run app in Docker. Configure connection to a DB using GO_SVC_TEMPLATE_DB_HOST and GO_SVC_TEMPLATE_DB_PORT
	-DOCKER_BUILDKIT=1 \
	docker run --name $(DB_CONTAINER_NAME) -d \
		-e POSTGRES_DB=go-svc-template \
		-e POSTGRES_PASSWORD=postgres \
		-p $(DB_PORT):5432 $(DB_IMAGE)
	DOCKER_BUILDKIT=1 \
	docker run --name $(CONTAINER_NAME) -t -d \
		-e GO_SVC_TEMPLATE_DB_PORT=$(DB_PORT) \
		-e GO_SVC_TEMPLATE_DB_HOST=host.docker.internal \
		-e GO_SVC_TEMPLATE_DB_SSL_MODE=disable \
		-e GO_SVC_TEMPLATE_SERVICE_SECRET=$(DUMMY_SECRET) \
		-e GO_SVC_TEMPLATE_HUMAN_READABLE_LOGS=true \
		--mount type=bind,source=$$(pwd)/test-config,target=/etc/shared-config \
		-p $(PORT):$(PORT) \
		$(DOCKER_IMAGE):$(APP_VERSION)

.PHONY: local-install li
local-install li:   ## Deploy to local Kubernetes (check kubectl context beforehand)
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

.PHONY: config-download
config-download:  ## download the JWT config file from k8s DEV
	@kubectl config use-context phdp-dev
	@mkdir -p test-config
	kubectl get cm shared-config -o jsonpath='{.data.config\.yaml}' > ./test-config/config.yaml
