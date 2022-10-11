#!/bin/bash
set -eo pipefail

# imports
source './utils.sh'

main() {
  # get the name of an IPFS Cluster pod
  crdName='ipfs-sample-1'
  labelValue="ipfs-cluster-${crdName}"
  labelName='app.kubernetes.io/name'
  ipfsClusterPodName=$(kubectl get pod -n "${NAMESPACE}" -l "${labelName}=${labelValue}" -o jsonpath='{.items[0].metadata.name}')
  
  # write a file to the ipfs-cluster container in the pod
  log "writing a file to ${ipfsClusterPodName}"
  kubectl exec -n "${NAMESPACE}" "${ipfsClusterPodName}" -c ipfs-cluster -- sh -c 'echo "hello from ${HOSTNAME} at $(date)" > /tmp/testfile.txt'
  myCID=$(kubectl exec -n "${NAMESPACE}" "${ipfsClusterPodName}" -c ipfs-cluster -- sh -c 'ipfs-cluster-ctl add /tmp/testfile.txt' | awk '{print $2}')
  
  # read the value
  ipfsClusterPodname2=$(kubectl get pod -n "${NAMESPACE}" -l "${labelName}=${labelValue}" -o jsonpath='{.items[1].metadata.name}')
  log "reading a file from ${ipfsClusterPodname2}"
  ipfsCommand="ipfs get --output /tmp/myfile.txt -- ${myCID}" 
  kubectl exec -n "${NAMESPACE}" "${ipfsClusterPodname2}" -c ipfs -- sh -c "${ipfsCommand}"
}

main