#!/bin/bash

set -e -o pipefail

#######################################
# Makes sure that the KIND command is 
# installed before continuing.
#
# Globals:
#  None
# Arguments:
#  None
# Returns:
#  None
#######################################
function check_kind() {
  if ! command -v kind >/dev/null 2>&1; then
    echo "kind is not installed. Please install kind before running this script."
    exit 1
  else 
    echo "kind is installed, ready to Go :)"
  fi
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
    echo "Failed to create metallb namespace"
    return 1
  fi

  # create the manifest
  if ! [[ $(kubectl apply -f "${metalLBURL}") ]]; then
    echo "Failed to create metallb manifest"
    return 1
  fi

  # wait for all pods with the label 'app=metallb' to be ready
  echo "📦 waiting for pods to be ready..."
  kubectl wait --for=condition=ready pod -l app=metallb -n "${metalLBNamespace}"
  echo "✅ pods are ready"

  # allocate a group of subnets to be used for the MetalLB instances
  local ipamConfig=$(docker network inspect -f '{{.IPAM.Config}}' kind)
  local subnet=$(echo "${ipamConfig}" | awk '{ print $1 }')
  local regex="([0-9]+)\.([0-9]+)\.([0-9]+)\.([0-9]+)/([0-9]+)"
  if ! [[ $subnet =~ $regex ]]; then
    echo "could not match"
  fi

  # create the subnets in MetalLB
  local net1="${BASH_REMATCH[1]}"
  local net2="${BASH_REMATCH[2]}"
  local net3="${BASH_REMATCH[3]}"
  local net4="${BASH_REMATCH[4]}"
  local subnetMask="${BASH_REMATCH[5]}"

  if ! [[ "${subnetMask}" -eq "16" ]]; then
    echo "subnet unsupported"
    return 1
  fi

  # create a single subnet
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
      - ${net1}.${net2}.255.200-${net1}.${net2}.255.250
END
}

#######################################
# Creates a new cluster with kind.
# Globals:
#  CLUSTER_NAME
# Arguments:
#  None
# Returns:
#  None
#######################################
function setup_kind_cluster() {
  # create the kind cluster if it doesn't already exist
  kind create cluster --name "${CLUSTER_NAME:-kind}"
}

# run commands or install
case "${1:-}" in
  "check")
    check_kind
    ;;
  "metallb")
    echo "installing metallb"
    install_metallb
    echo "✅ metallb installed"
    ;;
  *)
    setup_kind_cluster
    ;;
esac

# check_kind
# setup_kind_cluster
