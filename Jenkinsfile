pipeline {
  agent any

  options {
    timestamps()
    disableConcurrentBuilds()
  }

  environment {
    APP_NAME = 'distributed-task-queue'
    ACR_NAME = credentials('AZURE_ACR_NAME')
    ACR_LOGIN_SERVER = credentials('AZURE_ACR_LOGIN_SERVER')
    ACR_USERNAME = credentials('AZURE_ACR_USERNAME')
    ACR_PASSWORD = credentials('AZURE_ACR_PASSWORD')
    IMAGE_TAG = "${env.BUILD_NUMBER}"
  }

  stages {
    stage('Checkout') {
      steps {
        checkout scm
      }
    }

    stage('Unit Tests') {
      steps {
        sh 'cd api && go test ./...'
        sh 'cd worker && go test ./...'
      }
    }

    stage('UI Build') {
      steps {
        sh 'cd ui && npm ci && npm run build'
      }
    }

    stage('Docker Build') {
      steps {
        sh '''
          echo "TODO: add Dockerfiles and docker build commands"
          echo "api image: ${ACR_LOGIN_SERVER}/dtq-api:${IMAGE_TAG}"
          echo "worker image: ${ACR_LOGIN_SERVER}/dtq-worker:${IMAGE_TAG}"
          echo "ui image: ${ACR_LOGIN_SERVER}/dtq-ui:${IMAGE_TAG}"
        '''
      }
    }

    stage('Push to ACR') {
      steps {
        sh '''
          echo "TODO: replace with az acr login or docker login + docker push"
          echo "Using ACR login server ${ACR_LOGIN_SERVER}"
        '''
      }
    }

    stage('Deploy to AKS') {
      steps {
        sh '''
          echo "TODO: configure kubectl context for AKS"
          echo "TODO: apply infra/aks/base manifests with updated image tags"
        '''
      }
    }
  }

  post {
    success {
      echo 'Week 3 CI scaffold pipeline completed successfully.'
    }
    failure {
      echo 'Pipeline failed. Check stage logs for details.'
    }
  }
}
