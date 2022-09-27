#!/bin/sh
set -e -o pipefail
source './utils.sh'

log "verifying contents"

# get the name of an IPFS Cluster pod
crdName='cluster-1'
labelValue="ipfs-cluster-${crdName}"
labelName='app.kubernetes.io/name'
ipfsClusterPodName=$(kubectl get pod -n "${NAMESPACE}" -l "${labelName}=${labelValue}" -o jsonpath='{.items[0].metadata.name}')

# ensure that the corresponding IPFS container has pinned content
pins=$(kubectl exec "${ipfsClusterPodName}" -n "${NAMESPACE}" -c 'ipfs' -- ipfs)
