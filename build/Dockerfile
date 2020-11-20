# syntax = docker/dockerfile:experimental

# Args used in the image names must be defined before respective FROM lines
# Otherwise the following error will appear: rpc error: code = Unknown desc = invalid reference format
ARG GO_VERSION=1.15
ARG CILINT_VERSION=v1.32

# Base
FROM golang:${GO_VERSION}-alpine AS base
ARG GITHUB_USER_TOKEN

RUN apk update && apk upgrade && \
  apk --update add git gcc make ca-certificates musl-dev

# Add a user to run in non-root mode
RUN mkdir /user && \
    echo 'nobody:x:65534:65534:nobody:/:' > /user/passwd && \
    echo 'nobody:x:65534:' > /user/group

WORKDIR /app

RUN git config --global --add url."https://${GITHUB_USER_TOKEN}:x-oauth-basic@github.com/".insteadOf "https://github.com/"
ENV GOPRIVATE="github.com/gesundheitscloud/*"
ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64

COPY go.mod .
RUN --mount=type=cache,target=/go/pkg/mod \
  go mod download
COPY . .

FROM golangci/golangci-lint:${CILINT_VERSION} AS lint-base

FROM base AS lint
RUN --mount=target=. \
  --mount=from=lint-base,src=/usr/bin/golangci-lint,target=/usr/bin/golangci-lint \
  --mount=type=cache,target=/root/.cache/go-build \
  --mount=type=cache,target=/go/pkg/mod \
  --mount=type=cache,target=/root/.cache/golangci-lint \
  golangci-lint run -v --timeout 10m0s ./...

FROM base AS unit-test
# sqlite requires CGO
ARG GO_SVC_TEMPLATE_DB_PORT=5440
ARG GO_SVC_TEMPLATE_DB_HOST=172.17.0.1
ENV CGO_ENABLED=1
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    GO_SVC_TEMPLATE_DB_PORT=$(GO_SVC_TEMPLATE_DB_PORT) \
    GO_SVC_TEMPLATE_DB_HOST=$(GO_SVC_TEMPLATE_DB_HOST) \
    make test-verbose


FROM base AS build
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    make build

# Distribution
# `gcr.io/distroless/base:latest` is like `scratch` but Anchore can recognize it (Anchore does not recognize scratch as distro and issues a warning)
# What it contains: https://github.com/GoogleContainerTools/distroless/blob/master/base/README.md
FROM gcr.io/distroless/base:latest AS run-base
ARG GO_VERSION=undefined
ARG APP_VERSION=undefined
ARG GIT_COMMIT=undefined
ARG BUILD_DATE=undefined

COPY --from=base /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=base /user/group /user/passwd /etc/
COPY --from=base /app /app

LABEL org.label-schema.build-date="$BUILD_DATE"
LABEL org.label-schema.name="PLACEHOLDER"
LABEL org.label-schema.description="PLACEHOLDER"
LABEL org.label-schema.vcs-url="https://github.com/gesundheitscloud/PLACEHOLDER"
LABEL org.label-schema.vcs-ref="$GIT_COMMIT"
LABEL org.label-schema.vendor="data4life gGmbH"
LABEL org.label-schema.version="$APP_VERSION"
LABEL org.label-schema.schema-version="1.0"
LABEL go-version="$GO_VERSION"

WORKDIR /app

FROM run-base AS run
EXPOSE 8080
LABEL org.label-schema.description="Does this and that"
LABEL org.label-schema.name="go-svc-template"
COPY --from=build "/app/go-svc-template" /app
USER nobody:nobody
ENTRYPOINT ["/app/go-svc-template"]
