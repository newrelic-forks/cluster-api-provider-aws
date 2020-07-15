#!/bin/bash

# updates newrelic forks
function update {
    git fetch
    git checkout CONFAB-4271-EKS-image
    if git ls-remote richardcase; then
        echo "Branch already added locally. Continuing..."
    else
        echo "Richardcase's branch does not exist. Adding...."
        git remote add richardcase https://github.com/richardcase/cluster-api-provider-aws
    fi
    git merge richardcase/ekscontrolplane
    git merge eks-bootstrap
}

function build {
    # ensure relevant environment variables
    # example:
    #    export CAPA_DIRECTORY="$HOME/repos/cluster-api-provider-aws" && export CAPI_DIRECTORY="$HOME/repos/cluster-api/"

    [ ! -z "$CAPI_DIRECTORY" ] || { echo "Set the env var: CAPI_DIRECTORY to the path of the repo cluster-api"; exit 1; }
    [ ! -z "$CAPA_DIRECTORY" ] || { echo "Set the env var: CAPA_DIRECTORY to the path of the repo cluster-api-provider-aws"; exit 1; }

    # CAPI
    cd $CAPI_DIRECTORY
    CONTROLLER_IMG=cf-registry.nr-ops.net/cabpe-testing/cluster-api-controller  make docker-build-core
    [ -d "config/manager" ] && kustomize build config/manager | kubectl apply -f -

    # Make CAPA image
    LATEST_CABPE_SHA=$(git rev-parse HEAD)
    REGISTRY=cf-registry.nr-ops.net IMAGE_NAME=cabpe-testing/cluster-api-aws-controller:v0.0.0-$LATEST_CABPE_SHA make docker-build docker-push
    kustomize build config > infrastructure-components.yaml
    kubectl apply -f infrastructure-components.yaml
    kind load docker-image cf-registry.nr-ops.net/cabpe-testing/cluster-api-aws-controller-amd64:dev
    EKS_BOOTSTRAP_CONTROLLER_IMG=cf-registry.nr-ops.net/container-fabric/eks-bootstrap-controller make docker-build-eks-bootstrap
}

case "$1" in
        update)
            echo "Updating branch..."
            update
            ;;

        build)
            echo "Building images..."
            build
            ;;
        *)
            echo $"Usage: $0 {update|build}"
            exit 1
            ;;
esac
