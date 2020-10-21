#!groovy

@Library(value='pipeline-lib@v2.6.0', changelog=false) _

buildPipeline projectName: 'go-svc-template',
    namespace: 'default',
    dockerRegistryID: 'phdp',
    vaultBranch: 'phdp'
