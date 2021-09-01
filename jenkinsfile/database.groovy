#!groovy

@Library(value='pipeline-lib@v2.19.0', changelog=false) _

databasePipeline projectName: 'go-svc-template',
    vaultBranch: 'phdp'
