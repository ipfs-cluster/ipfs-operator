#!/bin/bash
current_shell() {
  currentShell=$(readlink /proc/"${$}"/exe)
  printf "using shell: %s\n" "${currentShell}"
  version=$("${currentShell}" --version)
  printf "shell version: %s\n" "${version}"
}
current_shell

# show all available shells
cat /etc/shells

set -eo pipefail
      
# imports
source './utils.sh'
      
log "verifying contents"
      
# get the name of an IPFS Cluster pod
crdName='cluster-1'
labelValue="ipfs-cluster-${crdName}"
labelName='app.kubernetes.io/name'
ipfsClusterPodName=$(kubectl get pod -n "${NAMESPACE}" -l "${labelName}=${labelValue}" -o jsonpath='{.items[0].metadata.name}')
      
# ensure that the corresponding IPFS container has pinned content
pins=$(kubectl exec "${ipfsClusterPodName}" -n "${NAMESPACE}" -c 'ipfs' -- ipfs)