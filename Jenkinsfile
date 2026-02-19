pipeline {
  agent any

  options {
    timestamps()
    disableConcurrentBuilds()
  }

  environment {
    ACR_LOGIN_SERVER = credentials('AZURE_ACR_LOGIN_SERVER')
    AZURE_CLIENT_ID = credentials('AZURE_CLIENT_ID')
    AZURE_CLIENT_SECRET = credentials('AZURE_CLIENT_SECRET')
    AZURE_TENANT_ID = credentials('AZURE_TENANT_ID')
    AZURE_SUBSCRIPTION_ID = credentials('AZURE_SUBSCRIPTION_ID')
    AZURE_AKS_RESOURCE_GROUP = credentials('AZURE_AKS_RESOURCE_GROUP')
    AZURE_AKS_CLUSTER_NAME = credentials('AZURE_AKS_CLUSTER_NAME')
    K8S_NAMESPACE = 'dtq'
    IMAGE_TAG = "${env.BUILD_NUMBER}"
  }

  stages {
    stage('Checkout') {
      steps {
        checkout scm
      }
    }

    stage('Validate CI Contracts') {
      steps {
        sh '''
          set -euo pipefail

          for cmd in go node npm docker az kubectl; do
            if ! command -v "${cmd}" >/dev/null 2>&1; then
              echo "missing required tool: ${cmd}"
              exit 1
            fi
          done

          for required_var in \
            ACR_LOGIN_SERVER \
            AZURE_CLIENT_ID \
            AZURE_CLIENT_SECRET \
            AZURE_TENANT_ID \
            AZURE_SUBSCRIPTION_ID \
            AZURE_AKS_RESOURCE_GROUP \
            AZURE_AKS_CLUSTER_NAME; do
            if [ -z "${!required_var:-}" ]; then
              echo "missing required Jenkins credential/env: ${required_var}"
              exit 1
            fi
          done

          echo "validated required tools and Jenkins credential contracts"
        '''
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
          set -euo pipefail

          for component in api worker ui; do
            image_ref="${ACR_LOGIN_SERVER}/dtq-${component}:${IMAGE_TAG}"
            docker build -f "${component}/Dockerfile" -t "${image_ref}" "${component}"
            echo "built ${image_ref}"
          done
        '''
      }
    }

    stage('Push to ACR') {
      steps {
        sh '''
          set -euo pipefail

          echo "${AZURE_CLIENT_SECRET}" | docker login "${ACR_LOGIN_SERVER}" --username "${AZURE_CLIENT_ID}" --password-stdin

          for component in api worker ui; do
            image_ref="${ACR_LOGIN_SERVER}/dtq-${component}:${IMAGE_TAG}"
            docker push "${image_ref}"
            echo "pushed ${image_ref}"
          done

          docker logout "${ACR_LOGIN_SERVER}" || true
        '''
      }
    }

    stage('Deploy to AKS') {
      steps {
        sh '''
          set -euo pipefail

          az login --service-principal \
            --username "${AZURE_CLIENT_ID}" \
            --password "${AZURE_CLIENT_SECRET}" \
            --tenant "${AZURE_TENANT_ID}" >/dev/null

          az account set --subscription "${AZURE_SUBSCRIPTION_ID}"
          az aks get-credentials \
            --resource-group "${AZURE_AKS_RESOURCE_GROUP}" \
            --name "${AZURE_AKS_CLUSTER_NAME}" \
            --overwrite-existing

          ACR_LOGIN_SERVER="${ACR_LOGIN_SERVER}" \
          IMAGE_TAG="${IMAGE_TAG}" \
          K8S_NAMESPACE="${K8S_NAMESPACE}" \
          bash infra/aks/scripts/deploy-release.sh
        '''
      }
    }
  }

  post {
    success {
      echo 'Week 3 CI pipeline completed: build, push, and AKS rollout succeeded.'
    }
    failure {
      echo 'Pipeline failed. Check stage logs for failing command output.'
    }
  }
}
