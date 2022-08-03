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
	kind create cluster --name "${CLUSTER_NAME:-kind}"
}

check_kind
setup_kind_cluster
