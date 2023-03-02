#!/bin/bash
set -eo pipefail


# imports
echo "sourcing utils.sh"
source './utils.sh'



main() {
  echo "this is the first message once the script begins to run"
  # get the name of an IPFS Cluster pod
  crdName='private-ipfs'
  labelValue="ipfs-cluster-${crdName}"
  labelName='app.kubernetes.io/name'
  ipfsClusterPodName=$(kubectl get pod -n "${NAMESPACE}" -l "${labelName}=${labelValue}" -o jsonpath='{.items[0].metadata.name}')
  
  # write a file to the ipfs-cluster container in the pod
  echo "writing a file to ${ipfsClusterPodName}"
  local results=$(kubectl exec -n "${NAMESPACE}" "${ipfsClusterPodName}" -c ipfs-cluster -- sh -c 'echo "hello from ${HOSTNAME} at $(date)" > /tmp/testfile.txt')
  echo "results of write: ${results}"
  echo "grabbing the content ID"
  myCID=$(kubectl exec -n "${NAMESPACE}" "${ipfsClusterPodName}" -c ipfs-cluster -- sh -c 'ipfs-cluster-ctl add /tmp/testfile.txt' | awk '{print $2}')
  echo "content ID is: ${myCID}"
  
  # read the value
  echo "getting the other ipfs cluster podname"
  ipfsClusterPodname2=$(kubectl get pod -n "${NAMESPACE}" -l "${labelName}=${labelValue}" -o jsonpath='{.items[1].metadata.name}')
  ipfsCommand="ipfs get --output /tmp/myfile.txt -- ${myCID}" 
  echo "reading a file from ${ipfsClusterPodname2} using command: '${ipfsCommand}'"
  kubectl exec -n "${NAMESPACE}" "${ipfsClusterPodname2}" -c ipfs -- sh -c "${ipfsCommand}"
  echo "success!"
}

main
