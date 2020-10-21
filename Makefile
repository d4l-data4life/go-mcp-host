# Common Variables
GIT_VERSION ?= $(shell git describe --tags --always --dirty)
APP_VERSION ?= $(GIT_VERSION)
COMMIT=$(shell git rev-parse --short HEAD)
BRANCH=$(shell git rev-parse --abbrev-ref HEAD)
USER=$(shell whoami)
BINARY=go-svc-template

# Go Variables
CILINT_VERSION := v1.25
PKG=github.com/gesundheitscloud/go-svc-template/pkg/config
LDFLAGS="-X '$(PKG).Version=${GIT_VERSION}' -X '$(PKG).Commit=${COMMIT}' -X '$(PKG).Branch=${BRANCH}' -X '$(PKG).BuildUser=${USER}'"
GOCMD=go
GOBUILD=$(GOCMD) build -ldflags $(LDFLAGS)
GOTEST=$(GOCMD) test
PUBLIC_KEY_PATH="$(shell pwd)/test-keys/jwtpublickey.pem"
SRC = cmd/api/*.go
DUMMY_SECRET=very-secure-secret
define LOCAL_VARIABLES
GO_SVC_TEMPLATE_JWT_PUBLIC_KEY_PATH=$(PUBLIC_KEY_PATH) \
GO_SVC_TEMPLATE_SERVICE_SECRET=$(DUMMY_SECRET)
endef

# Docker Variables
DOCKER_REGISTRY ?= phdp-snapshots.hpsgc.de
DOCKER_IMAGE=$(DOCKER_REGISTRY)/$(BINARY)
CONTAINER_NAME=$(BINARY)
PORT=8080
DB_IMAGE=postgres
DB_CONTAINER_NAME=$(BINARY)-postgres
DB_PORT=5432

# Deploy variables
APP ?= $(BINARY)
KUBECONFIG ?= $(HOME)/.kube/config
ENV_YAML ?= "$(shell pwd)/deploy/config/local.yaml"
SECRETS_YAML ?= "$(shell pwd)/deploy/local/secrets.yaml"
NAMESPACE ?= default

.PHONY: help clean test unit-test lint docker-build db docker-push twistlock-scan deploy local-test lt local-race lr test-verbose vendor build run docker-database docker-run dr local-install li local-delete ld docker-prune

## ----------------------------------------------------------------------
## Help: Makefile for app: go-svc-template
## ----------------------------------------------------------------------

help:               ## Show this help (default)
	@grep -E '^[a-zA-Z._-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

version:              ## Display current version
	@echo $(APP_VERSION)

## ----------------------------------------------------------------------
## Mandatory for PHDP Jenkins
## ----------------------------------------------------------------------
parallel-test:               ## Specifies space-separated list of targets that can be run in parallel
	@echo "lint unit-test-postgres"

clean:              ## Remove compiled binary, versioned chart and docker containers
	rm -f $(BINARY)
	rm -f deploy/helm-chart/Chart.yaml
	-docker rm -f $(CONTAINER_NAME)
	-docker rm -f $(DB_CONTAINER_NAME)

test: lint unit-test-postgres  ## Run all test activities sequentially

unit-test-postgres: docker-database unit-test docker-database-delete         ## Run unit tests inside the Docker and uses Postgres DB

unit-test:          ## Run unit tests inside the Docker image
	docker build \
		--build-arg GITHUB_USER_TOKEN \
		-t $(DOCKER_IMAGE):test \
		-f build/test.Dockerfile \
		.
	docker run --rm \
		-e GO_SVC_TEMPLATE_DB_PORT=$(DB_PORT) \
		-e GO_SVC_TEMPLATE_DB_HOST=172.17.0.1 \
		$(DOCKER_IMAGE):test

lint:
	docker build \
		--build-arg CILINT_VERSION=${CILINT_VERSION} \
		--build-arg GITHUB_USER_TOKEN \
		-t "$(DOCKER_IMAGE):lint" \
		-f build/lint.Dockerfile \
		.
	docker run --rm "$(DOCKER_IMAGE):lint"

docker-build db:    ## Build Docker image
	docker build \
		--build-arg GITHUB_USER_TOKEN \
		-t $(DOCKER_IMAGE):$(APP_VERSION) \
		-f build/Dockerfile \
		.

docker-push:        ## Push Docker image to registry
	docker push $(DOCKER_IMAGE):$(APP_VERSION)

scan-docker-images:
	@echo "$(DOCKER_IMAGE):$(APP_VERSION)"

deploy: helm-deploy reload     ## Redeploy and reload secrets

reload:   ## Reloads configuration - currently: gracefully restarts K8s deployment
	kubectl -n $(NAMESPACE) rollout restart deployment.apps/$(APP)

helm-deploy:   ## Deploy to kubernetes using Helm
	sed -e 's/<github_release>/$(APP_VERSION)/g' deploy/helm-chart/Chart.versionless.yaml > deploy/helm-chart/Chart.yaml
	helm upgrade --install $(APP) \
		-f $(ENV_YAML) \
		-f $(SECRETS_YAML) \
		--namespace $(NAMESPACE) \
		--set imageTag=$(DOCKER_IMAGE):$(APP_VERSION) \
		--wait \
		deploy/helm-chart

## ----------------------------------------------------------------------
## Additional
## ----------------------------------------------------------------------

local-test lt:      ## Run tests natively
	$(LOCAL_VARIABLES) \
	$(GOTEST) -timeout 15s -cover -covermode=atomic ./...

test-verbose:       ## Run tests natively in verbose mode
	$(LOCAL_VARIABLES) \
	$(GOTEST) -timeout 15s -cover -covermode=atomic -v ./...

vendor:             ## Download and tidy go depenencies
	@go mod tidy

build:              ## Build app
	$(GOBUILD) -o $(BINARY) $(SRC)

run:                ## Run app natively
	$(LOCAL_VARIABLES) \
	$(GOCMD) run $(SRC)

docker-database:    ## Run database in Docker
	docker run --name $(DB_CONTAINER_NAME) -d \
		-e POSTGRES_DB=go-svc-template \
		-e POSTGRES_PASSWORD=postgres \
		-p $(DB_PORT):5432 $(DB_IMAGE)

docker-database-delete: ## Delete database in Docker
	-docker rm -f $(DB_CONTAINER_NAME)

docker-run dr:      ## Run app in Docker. Configure connection to a DB using GO_SVC_TEMPLATE_DB_HOST and GO_SVC_TEMPLATE_DB_PORT
	-docker run --name $(DB_CONTAINER_NAME) -d \
		-e POSTGRES_DB=go-svc-template \
		-e POSTGRES_PASSWORD=postgres \
		-p $(DB_PORT):5432 $(DB_IMAGE)
	docker run --name $(CONTAINER_NAME) -t -d \
		-e GO_SVC_TEMPLATE_DB_PORT=$(DB_PORT) \
		-e GO_SVC_TEMPLATE_DB_HOST=host.docker.internal \
		-e GO_SVC_TEMPLATE_SERVICE_SECRET=$(DUMMY_SECRET) \
		--mount type=bind,source=$$(pwd)/test-keys,target=/keys \
		-p $(PORT):$(PORT) \
		$(DOCKER_IMAGE):$(APP_VERSION)

local-install li:   ## Deploy to local Kubernetes (check kubectl context beforehand)
	kubectl config use-context docker-desktop
	make deploy

local-delete ld:    ## Delete local helm deployment
	kubectl config use-context docker-desktop
	helm delete $(APP) --namespace ${NAMESPACE}

docker-prune:       ## Delete local docker images of this repo except for the current version
	docker images | grep $(DOCKER_IMAGE) | grep -v $(APP_VERSION) | awk '{print $$3}' | xargs docker rmi -f
	# Removes all exited docker containers
	docker ps --quiet --all --filter status=exited | xargs docker rm
