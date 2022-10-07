#!/bin/bash
set -eo pipefail
      
# imports
source './utils.sh'

########################################
# Count the amount of pins in the IPFS node
# Arguments:
#   Name of IPFS pod
# 	Name of follow container
# Globals:
#   NAMESPACE 
# Returns:
#   Number of pinned nodes
########################################
count_pins() {
	declare podName="${1}"
	declare followContainerName="${2}"
	declare -i numPins=0
	declare pinType=""
	declare pinTypeRecursive="recursive"

	# ensure that the corresponding IPFS container has pinned content
  pins=$(kubectl exec "${podName}" -n "${NAMESPACE}" -c "${followContainerName}" -- ipfs pin ls | grep -i "${pinTypeRecursive}")
	readarray -d $'\n' -t pinArray <<< "${pins}"
	echo "${#pinArray[*]}"
}

main() {
  log "verifying contents"

	# default number of pins
	declare -i DEFAULT_PINS=2
	
  # get the name of an IPFS Cluster pod
  declare crdName='cluster-1'
  declare labelValue="ipfs-cluster-${crdName}"
  declare labelName='app.kubernetes.io/name'
  declare ipfsClusterPodName=$(kubectl get pod -n "${NAMESPACE}" -l "${labelName}=${labelValue}" -o jsonpath='{.items[0].metadata.name}')

	# get number of pins:
	declare -i numPins=0
	while [[ "${numPins}" -le "${DEFAULT_PINS}" ]]; do
		numPins=$(count_pins "${ipfsClusterPodName}")
		log "current number of pins: ${numPins}"
		if [[ "${numPins}" -gt "${DEFAULT_PINS}" ]]; then 
			break
		else
			log "not enough pins, sleeping..."
			sleep 5
		fi
	done
}

main