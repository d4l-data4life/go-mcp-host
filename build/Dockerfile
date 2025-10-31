# syntax = docker/dockerfile:experimental

# Args used in image names must be defined befrore respecive FROM lines
# Otherwise the following error will appear: rpc error: code = Unknown desc = invalid reference format

# Providing local IMG_BASE allows to save on API calls to Dockerhub (these are limited)
# Set empty to use the upstream
ARG IMG_BASE=
ARG GO_VERSION=1.24

########################
### Base container
########################

FROM ${IMG_BASE}golang:${GO_VERSION}-alpine AS base
ARG GITHUB_USER_TOKEN

RUN apk update && apk upgrade && \
  apk --update add git gcc make ca-certificates musl-dev

# Add root-CA certificate for DB connection
COPY root.ca.pem /root.ca.pem

# Add a user to run in non-root mode
RUN mkdir /user && \
  echo 'nobody:x:65534:65534:nobody:/:' > /user/passwd && \
  echo 'nobody:x:65534:' > /user/group

WORKDIR /app

RUN git config --global --add url."https://${GITHUB_USER_TOKEN}:x-oauth-basic@github.com/".insteadOf "https://github.com/"
ENV GOPRIVATE="github.com/d4l-data4life/*"
ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64

COPY go.mod .
RUN --mount=type=cache,target=/go/pkg/mod \
  go mod download
COPY . .

########################
### Building
########################

FROM base AS build
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    make build

########################
### Running
########################

# Distribution
# `gcr.io/distroless/base:latest` is like `scratch` but Anchore can recognize it (Anchore does not recognize scratch as distro and issues a warning)
# What it contains: https://github.com/GoogleContainerTools/distroless/blob/master/base/README.md
FROM gcr.io/distroless/base:latest AS run-base
ARG APP_VERSION=undefined
ARG GIT_COMMIT=undefined
ARG BUILD_DATE=undefined

COPY --from=base /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=base /user/group /user/passwd /etc/
COPY --from=base --chown=nobody:nobody /app /app

LABEL org.label-schema.build-date="$BUILD_DATE"
LABEL org.label-schema.vcs-url="https://github.com/d4l-data4life/go-mcp-host"
LABEL org.label-schema.vcs-ref="$GIT_COMMIT"
LABEL org.label-schema.vendor="data4life gGmbH"
LABEL org.label-schema.version="$APP_VERSION"
LABEL org.label-schema.schema-version="1.0"
LABEL go-version="$GO_VERSION"

WORKDIR /app

# Note on run-* targets:
# ARG variables will not work properly when used in:
# COPY --from "run-${VAR}"
# ENTRYPOINT "/sth/${VAR}/bin"

FROM run-base AS run
ARG PORT=8080
EXPOSE $PORT
LABEL org.label-schema.name="go-mcp-host"
LABEL org.label-schema.description="Does this and that"
COPY --from=base /root.ca.pem /root.ca.pem
COPY --from=build /app/go-mcp-host /app
USER nobody:nobody
ENTRYPOINT ["/app/go-mcp-host"]
