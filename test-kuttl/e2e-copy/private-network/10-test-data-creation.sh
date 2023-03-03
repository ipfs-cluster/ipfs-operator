#!/bin/bash
set -eo pipefail

echo "whoami: $(whoami)"

# imports
echo "sourcing utils.sh"
source './utils.sh'


echo "checking to see if awk is installed: '$(which awk)'"
echo "checking if grep is installed: '$(which grep)'"

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

  echo "list out the contents of temp, check for testfile"
  results=$(kubectl exec -n "${NAMESPACE}" "${ipfsClusterPodName}" -c ipfs-cluster -- sh -c 'ls -al /tmp | grep testfile')
  echo "results: '${results}'"
  echo "contents of new file:"
  local contents=$(kubectl exec -n "${NAMESPACE}" "${ipfsClusterPodName}" -c ipfs-cluster -- sh -c 'cat /tmp/testfile.txt')
  echo "file contents: '${contents}'"
  echo "checking for ipfs-cluster-ctl"
  local ipfsClusterCMD=$(kubectl exec -n "${NAMESPACE}" "${ipfsClusterPodName}" -c ipfs-cluster -- sh -c 'which ipfs-cluster-ctl')
  echo "ipfs-cluster-ctl: ${ipfsClusterCMD}"
  echo "checking permissions to use ipfs-cluster-ctl:"
  local ipfsClusterCtlPerms=$(kubectl exec -n "${NAMESPACE}" "${ipfsClusterPodName}" -c ipfs-cluster -- sh -c 'ls -al /usr/local/bin/ipfs-cluster-ctl')
  echo "perms: ${ipfsClusterCtlPerms}"
  echo "checking container user:"
  local ctrUser=$(kubectl exec -n "${NAMESPACE}" "${ipfsClusterPodName}" -c ipfs-cluster -- sh -c 'whoami')
  echo "continer user: ${ctrUser}"

  # delete the lockfile if it exists
  kubectl exec -n "${NAMESPACE}" "${ipfsClusterPodName}" -c ipfs-cluster -- sh -c 'if [ -e /data/ipfs/repo.lock ]; then rm /data/ipfs/repo.lock; fi'


  # this fails
  echo "grabbing the content ID"
  myCID=$(kubectl exec -n "${NAMESPACE}" "${ipfsClusterPodName}" -c ipfs-cluster -- sh -c 'if [ -e /data/ipfs/repo.lock ]; then rm /data/ipfs/repo.lock; fi; ipfs-cluster-ctl add /tmp/testfile.txt' | awk '{print $2}')
  echo "content ID is: ${myCID}"
  
  # read the value
  echo "getting the other ipfs cluster podname"
  ipfsClusterPodname2=$(kubectl get pod -n "${NAMESPACE}" -l "${labelName}=${labelValue}" -o jsonpath='{.items[1].metadata.name}')
  
  # delete the lockfile if it exists
  echo "deleting the other lockfile now"
  kubectl exec -n "${NAMESPACE}" "${ipfsClusterPodname2}" -c ipfs-cluster -- sh -c 'if [ -e /data/ipfs/repo.lock ]; then rm /data/ipfs/repo.lock; fi'
  echo "checking to see if lockfile still exists"
  results=$(kubectl exec -n "${NAMESPACE}" "${ipfsClusterPodname2}" -c ipfs-cluster -- sh -c 'ls -al /data/ipfs' | grep 'repo.lock')
  echo "lockfile: ${results}"

  ipfsCommand="if [ -e /data/ipfs/repo.lock ]; then rm /data/ipfs/repo.lock; fi; ipfs get --output /tmp/myfile.txt -- ${myCID}" 
  echo "reading a file from ${ipfsClusterPodname2} using command: '${ipfsCommand}'"
  kubectl exec -n "${NAMESPACE}" "${ipfsClusterPodname2}" -c ipfs -- sh -c "${ipfsCommand}"
  echo "success!"
}

main
