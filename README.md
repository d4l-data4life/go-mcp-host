# Template for Go projects

## Repo setup

1. Use this repo as a [template](https://help.github.com/en/articles/creating-a-repository-from-a-template) for your new repository
2. Add the repo to the automatic configuration tool [phdp-release-it](https://github.com/gesundheitscloud/phdp-release-it)
    - [Repo list](https://github.com/gesundheitscloud/phdp-release-it/blob/master/repos.csv)
    - [Codeowner settings](https://github.com/gesundheitscloud/phdp-release-it/blob/master/CODEOWNERS.csv)
    - run the setup via tool `d4l setup-repo <repo-name> -r ./repos.csv -c ./CODEOWNERS_ret.csv`
3. Adjust the GitHub Actions in this repo to the correct needs and read [github-actions](https://github.com/gesundheitscloud/github-actions) for general understanding

## Template usage

1. replace `go-mcp-host` with the name of your service everywhere
1. replace `GO_SVC_TEMPLATE` with the capitalized version of your service
1. make sure the go.mod file looks reasonable
1. Choose a different PORT for the server (change 9000 to avoid conflicts)
1. Choose a different PORT for the local database (change 6000 to avoid conflicts)
1. Add description in `deploy/helm-chart/Chart.versionless.yaml`
1. Delete this part of the README
1. Happy Coding!

# `go-mcp-host`

This is the backend service providing providing some functionality.

:construction: :construction: :construction:

## Building, Running, Testing

The service can be run as ready-made docker containers for running in the background or via local go execution.
The Makefile provides a lot of helpful commands, to see a list run `make help`.

### Docker Containers

First clean old artifacts, then export your token for the private dependencies and build the docker containers and run them.

```bash
make clean
export GITHUB_USER_TOKEN=<your-token>
make docker-build
make docker-run
```

### Local execution

Export github user token before installing dependencies to be able to get private repositories.
Afterwards start a DB in docker and then run the service.

```bash
export GITHUB_USER_TOKEN=<your-GH-API-token>
go mod install
make docker-database
make run
```

### Tests in VSCode

To run the tests in VSCode the environment variables and (optionally) a Postgres DB have to be provided.
For that purpose you need to execute the following:

```bash
make .env
make docker-database
```

#### `.vscode/settings.json`

```json
{
    "go.testEnvFile": "${workspaceFolder}/.env"
}
```

### Linting

Linting requires `golangci-lint` to be installed.
https://golangci-lint.run/welcome/install/
Ideally also setup you VSCode to use this linter for go files.
Add `"go.lintTool": "golangci-lint",` in `settings.json`.

### Run in local Kubernetes

To be able to reach the running service add `go-mcp-host-dns.local` to `/etc/hosts' file.

```txt
127.0.0.1 kubernetes.docker.internal go-mcp-host-dns.local
```

```bash
export GITHUB_USER_TOKEN=<your-GH-API-token>
make docker-build
make local-install
```

## How to use

Refer to [Example API calls](example-API-calls.http) for selected examples of API calls.
This file requires [REST Client VSCode plugin](https://marketplace.visualstudio.com/items?itemName=humao.rest-client).

## Swagger API definition

The API specification can be found in `/swagger/api.yml`. To preview the specification:

1. add a [swagger viewer](https://marketplace.visualstudio.com/items?itemName=Arjun.swagger-viewer) to VSCode
1. open the `yml` file
1. open the preview with `SHIFT + OPTION + P`
