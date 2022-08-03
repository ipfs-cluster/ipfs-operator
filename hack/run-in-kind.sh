#!/bin/sh

set -e -o pipefail

# Declare globals
KIND_TAG="local-build"
DOCKER_IMAGE="quay.io/redhat-et-ipfs/ipfs-operator"

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

# build the container image
make docker-build
docker tag "${DOCKER_IMAGE}:latest" "${DOCKER_IMAGE}:${KIND_TAG}"
kind load docker-image "${DOCKER_IMAGE}:${KIND_TAG}"

# using helm, install the IPFS Cluster Operator into the current cluster
helm upgrade --install \
  --debug \
	--set image.tag="${KIND_TAG}" \
	ipfs-cluster ./helm/ipfs-operator

