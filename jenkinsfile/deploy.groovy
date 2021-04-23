#!groovy

@Library(value='pipeline-lib@v2.14.0', changelog=false) _

deployPipeline projectName: 'go-svc-template',
    namespace: 'default',
    dockerRegistryID: 'phdp',
    slackChannelPrefix: 'phdp',
    vaultBranch: 'phdp'
