#!/bin/bash

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

######################################
# Prints out the given message to STDOUT
# with timestamped formatting.
# Globals:
#  None
# Arguments:
#  msg: (string) message to print
# Returns:
#  None
######################################
function log() {
	local msg="$1"
	# get the current time
	local time=$(date +%Y-%m-%dT%H:%M:%S%z)
	# print the message with printf
	printf "[%s] %s\n" "${time}" "${msg}"
}

######################################
# Installs MetalLB into the default namespace
# for the current cluster.
# Globals:
#  None
# Arguments:
#  None
# Returns:
#  1 if MetalLB is not installed, 0 otherwise
######################################
function install_metallb() {
  local metalLBVersion='v0.12.1'
  local metalLBManifests="https://raw.githubusercontent.com/metallb/metallb/${metalLBVersion}/manifests"
  local metalLBNamespaceURL="${metalLBManifests}/namespace.yaml"
  local metalLBURL="${metalLBManifests}/metallb.yaml"
  local metalLBNamespace="metallb-system"

  # create the metallb namespace
  if ! [[ $(kubectl apply -f "${metalLBNamespaceURL}") ]]; then
    log "Failed to create metallb namespace"
    return 1
  fi

  # create the manifest
  if ! [[ $(kubectl apply -f "${metalLBURL}") ]]; then
    log "Failed to create metallb manifest"
    return 1
  fi

  # wait for all pods with the label 'app=metallb' to be ready
	# HACK: find a way to wait on the parent condition for the pods instead of
	#       waiting 30 seconds before trying to wait on the pods themselves.
  log "💤 Sleeping for 30 seconds to allow the metallb namespace to be initialized"
  sleep 30
  log "📦 Waiting for pods to be ready..."
  kubectl wait --for=condition=ready pod -l app=metallb -n "${metalLBNamespace}"

  # allocate a group of subnets to be used for the MetalLB instances
  log "📢 Allocating network addresses"
  local ipamConfig=$(docker network inspect -f '{{.IPAM.Config}}' kind)
  local subnet=$(echo "${ipamConfig}" | awk '{ print $1 }')
  local regex="([0-9]+)\.([0-9]+)\.([0-9]+)\.([0-9]+)/([0-9]+)"
  if ! [[ $subnet =~ $regex ]]; then
    log "could not match"
    return 1
  fi

  # create the subnets in MetalLB
  local net1="${BASH_REMATCH[1]}"
  local net2="${BASH_REMATCH[2]}"
  local net3="${BASH_REMATCH[3]}"
  local net4="${BASH_REMATCH[4]}"
  local subnetMask="${BASH_REMATCH[5]}"

  if ! [[ "${subnetMask}" -eq "16" ]]; then
    log "subnet unsupported"
    return 1
  fi

  # create a single subnet, grant 255 addresses to LB
  cat <<END | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  namespace: metallb-system
  name: config
data:
  config: |
    address-pools:
    - name: default
      protocol: layer2
      addresses:
      - ${net1}.${net2}.255.0-${net1}.${net2}.255.255
END
}
