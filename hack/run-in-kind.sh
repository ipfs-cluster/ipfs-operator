#! /bin/bash

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

# load images into kind
IMAGES=(
	"quay.io/redhat-et-ipfs/ipfs-operator"
	"quay.io/redhat-et-ipfs/ipfs-cluster"
)

# check if the --skip-build argument was provided
skipBuild=false
for arg in "$@"; do
	if [ "$arg" == "--skip-build" ]; then
		skipBuild=true
	fi
done

# build the two images
if [[ "$skipBuild" == false ]]; then
	make docker-build
	make -C ipfs-cluster-image image
fi

# tag the latest images
for i in "${IMAGES[@]}"; do
	docker tag "${i}:latest" "${i}:${KIND_TAG}"
	kind load docker-image "${i}:${KIND_TAG}"
done

# using helm, install the IPFS Cluster Operator into the current cluster
helm upgrade --install \
  --debug \
	--set image.tag="${KIND_TAG}" \
	--set ipfsCluster.tag="${KIND_TAG}" \
	--wait --timeout=300s \
	ipfs-cluster ./helm/ipfs-operator

# TODO: implement auto-deletion of previous operator pod
# # if there is an existing operator pod running, delete it so we can properly restart
# kubectl delete pod -l app=ipfs-operator