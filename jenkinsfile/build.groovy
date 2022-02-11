#!groovy

@Library(value='pipeline-lib@v2', changelog=false) _

buildPipeline projectName: 'go-svc-template',
    dockerRegistryID: 'phdp',
    vaultBranch: 'phdp',
    buildJobForDockerImageScan: 'scanDockerImages'
