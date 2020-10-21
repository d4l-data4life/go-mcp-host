# Tester
ARG GO_VERSION=1.15
FROM golang:${GO_VERSION}-alpine

RUN apk update && apk upgrade && \
  apk --update --no-cache add git gcc musl-dev make tzdata && \
  mkdir /app

WORKDIR /app

ENV GO111MODULE=on
ENV GOPRIVATE="github.com/gesundheitscloud/*"

ARG GITHUB_USER_TOKEN
RUN git config --global --add url."https://${GITHUB_USER_TOKEN}:x-oauth-basic@github.com/".insteadOf "https://github.com/"

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .

CMD GOOS=linux GOARCH=amd64 make test-verbose
