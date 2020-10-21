ARG CILINT_VERSION

FROM golangci/golangci-lint:${CILINT_VERSION}

# Set the working directory outside $GOPATH to enable the support for modules.
WORKDIR /src

# Import the code from the context.
COPY ./ ./

# Use github token auth to be able to access internal repositories
ARG GITHUB_USER_TOKEN
RUN git config --global url."https://${GITHUB_USER_TOKEN}:x-oauth-basic@github.com/".insteadOf "https://github.com/"

# Set the environment variables for the `go` command.
ENV GOPRIVATE="github.com/gesundheitscloud/*"
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64

CMD ["golangci-lint", "run", "-v"]
