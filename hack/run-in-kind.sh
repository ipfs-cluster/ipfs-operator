#!/bin/bash

set -e -o pipefail

# Declare globals
KIND_TAG="local-build"

################################
# Makes sure the given commands are installed
# before continuing.
# Globals:
#   None
# Arguments:
#   args: (list) commands to check for
# Returns:
#  0 if all commands are installed, 1 otherwise
################################
function check_cmd() {
	for cmd in "$@"; do
		if ! command -v $cmd >/dev/null 2>&1; then
			echo "Error: $cmd is not installed"
			exit 1
		fi
	done
}

# make sure that helm, kind, and docker are installed
check_cmd helm docker kind

# build the container images
make docker-build
make -C ipfs-cluster-image image

# load them into kind
IMAGES=(
	"quay.io/redhat-et-ipfs/ipfs-operator"
	"ipcr.io/ipfs-cluster-k8s-image"
)

for i in "${IMAGES[@]}"; do
	docker tag "${i}:latest" "${i}:${KIND_TAG}"
	kind load docker-image "${i}:${KIND_TAG}"
done

# using helm, install the IPFS Cluster Operator into the current cluster
helm upgrade --install \
  --debug \
	--set image.tag="${KIND_TAG}" \
	--set ipfsCluster.tag="${KIND_TAG}" \
	ipfs-cluster ./helm/ipfs-operator

# TODO: implement auto-deletion of previous operator pod
# # if there is an existing operator pod running, delete it so we can properly restart
# kubectl delete pod -l app=ipfs-operator