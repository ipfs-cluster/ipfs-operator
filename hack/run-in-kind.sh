#! /bin/bash

set -e -o pipefail

# Declare globals
if ! [[ "${KIND_TAG}" == "" ]]; then
	echo "❗ Using KIND_TAG: ${KIND_TAG} ❗"
fi

KIND_TAG="${KIND_TAG:-local-build}"


# process arguments
skipBuild="false"
skipTag="false"

for arg in "$@"; do
	case "${arg}" in
	  "--skip-build")
			skipBuild="true"
			;;
		"--skip-tag")
			skipTag="true"
			;;
	esac
done


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
)


# build the two images
if [[ "${skipBuild}" == false ]]; then
	make docker-build
	make -C ipfs-cluster-image image
else
	echo "⏩ skipping build"
fi

# tag the latest images
if [[ "${skipTag}" == false ]]; then
	for i in "${IMAGES[@]}"; do
		docker tag "${i}:latest" "${i}:${KIND_TAG}"
		kind load docker-image "${i}:${KIND_TAG}"
	done
else
  echo "⏩ skipping tag"
fi


# using helm, install the IPFS Cluster Operator into the current cluster
helm upgrade --install \
  --debug \
	--set image.tag="${KIND_TAG}" \
	--wait --timeout=300s \
	ipfs-cluster ./helm/ipfs-operator

# TODO: implement auto-deletion of previous operator pod
# # if there is an existing operator pod running, delete it so we can properly restart
# kubectl delete pod -l app=ipfs-operator