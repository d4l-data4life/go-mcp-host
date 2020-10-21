# Builder
ARG GO_VERSION=1.15
FROM golang:${GO_VERSION}-alpine AS builder

RUN apk update && apk upgrade && \
  apk --update add git gcc make

WORKDIR /app

ENV GO111MODULE=on
ENV GOPRIVATE="github.com/gesundheitscloud/*"

ARG GITHUB_USER_TOKEN
RUN git config --global --add url."https://${GITHUB_USER_TOKEN}:x-oauth-basic@github.com/".insteadOf "https://github.com/"

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 make build

# Distribution
FROM alpine:latest

RUN apk update && apk upgrade && \
  apk --update --no-cache add tzdata && \
  mkdir /app

WORKDIR /app

EXPOSE 9124

COPY --from=builder /app/VERSION /app
COPY --from=builder /app/go-svc-template /app

# Add a user to run nginx in non-root mode
RUN addgroup -S go-svc-template && adduser -S go-svc-template -G go-svc-template
RUN chown -R go-svc-template:go-svc-template /app/go-svc-template

USER go-svc-template

CMD /app/go-svc-template
