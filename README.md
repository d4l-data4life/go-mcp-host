# Template for Go projects

## Repo setup
1. Use this repo as a [template](https://help.github.com/en/articles/creating-a-repository-from-a-template) for your new repository
2. Add the repo to the automatic configuration tool [phdp-release-it](https://github.com/gesundheitscloud/phdp-release-it)
    - [Repo list](https://github.com/gesundheitscloud/phdp-release-it/blob/master/repos_ret.csv)
    - [Codeowner settings](https://github.com/gesundheitscloud/phdp-release-it/blob/master/CODEOWNERS_ret.csv)
    - run the setup via tool `d4l setup-repo <repo-name> -c ./CODEOWNERS_ret.csv`
3. Follow the instructions in [phdp-jenkins-config](https://github.com/gesundheitscloud/phdp-jenkins-config) to register your repo for build and deploy

## Template usage
1. replace `go-svc-template` with the name of your service everywhere
1. replace `GO_SVC_TEMPLATE` with the capitalized version of your service
1. make sure the go.mod file looks reasonable
1. Choose a different PORT for the server (change 9000 to avoid conflicts)
1. Choose a different PORT for the local database (change 6000 to avoid conflicts)
1. Add description in `deploy/helm-chart/Chart.versionless.yaml`
1. Delete this part of the README
1. Happy Coding!

# `go-svc-template`

This is the backend service providing providing some functionality.

:construction: :construction: :construction:

## Building, Running, Testing

The Makefile provides a lot of helpful commands, to see a list run `make help`.

### Local execution

```bash
export GITHUB_USER_TOKEN=<your-GH-API-token>
make build
make run
make test
```

## Docker builds

- connect to VPN
- login to nexus docker images repository using your nexus credentials
    ```
    docker login phdp-snapshots.hpsgc.de
    ```
- run docker commands from makefile

### Test Execution in VSCode

To run the tests in VSCode the environment variables have to be provided.

#### `.vscode/settings.json`

```json
{
    "go.testEnvFile": "${workspaceFolder}/.vscode/.env"
}
```

#### `.vscode/.env`

```bash
GO_SVC_TEMPLATE_SERVICE_SECRET=very-secure-secret
GO_SVC_TEMPLATE_HUMAN_READABLE_LOGS=true
GO_SVC_TEMPLATE_TEST_WITH_DB=false
GO_SVC_TEMPLATE_DB_SSL_MODE=disable
```

For other options see `make help`

## Ops checks

Liveness and Readiness checks are meant for Kubernetes.

- Liveness is a check that responds with HTTP code 200 if the application has started
- Readiness is a check that responds with HTTP code 200 if the application is ready to serve requests (e.g., connected to the database)

## Run in local Kubernetes

1. **Add localhost alias**

    Add `go-svc-template-dns.local` to `/etc/hosts' file. For example in the kubernetes.docker section:

    ```txt
    # To allow the same kube context to work on the host and the container:
    127.0.0.1 kubernetes.docker.internal go-svc-template-dns.local
    # End of section
    ```

1. **Build the image**

    ```bash
    export GITHUB_USER_TOKEN=<your-GH-API-token>
    make docker-build
    ```

1. **Deploy to local Kubernetes**

    - make sure you have the right `kubectl` context selected (by default docker-desktop)

        ```bash
        kubectl config use-context docker-desktop
        ```

    - render templates and deploy manifests

        ```bash
        make kube-deploy
        ```

    - check that the pod is running

        ```bash
        kubectl get pod
        ```

## How to use

Refer to [Example API calls](example-API-calls.http) for selected examples of API calls.
This file requires [REST Client VSCode plugin](https://marketplace.visualstudio.com/items?itemName=humao.rest-client).

## Swagger API definition

The API specification can be found in `/swagger/api.yml`. To preview the specification:

1. add a [swagger viewer](https://marketplace.visualstudio.com/items?itemName=Arjun.swagger-viewer) to VSCode
1. open the `yml` file
1. open the preview with `SHIFT + OPTION + P`
