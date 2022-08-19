#!/bin/sh
user=ipfs

# This is a custom entrypoint for k8s designed to connect to the bootstrap
# node running in the cluster. It has been set up using a configmap to
# allow changes on the fly.

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
log() {
	msg="$1"
	# get the current time
	time=$(date +%Y-%m-%dT%H:%M:%S%z)
	# print the message with printf
	printf "[%s] %s\n" "${time}" "${msg}"
}

######################################
# Prints out information about the running
# container for debugging purposes.
# Globals:
#  version: (string) version of the container
# Arguments:
#  None
# Returns:
#  None
######################################
print_container_info() {
	log "---------- IPFS Cluster container version: ${version:-unknown} ----------"
	log "${@}"
}



######################################
# This is a custom entrypoint for k8s designed to connect to the bootstrap
# node running in the cluster. It has been set up using a configmap to
# allow changes on the fly.
#
# Globals:
#  BOOTSTRAP_PEER_ID (string) the peer id of the bootstrap node
#  BOOTSTRAP_PEER_PRIV_KEY (string) the private key of the bootstrap node
#  BOOTSTRAP_ADDR (string) the address of the bootstrap node
#  SVC_NAME (string) the name of the service to connect to
######################################
run_ipfs_cluster() {
	if [ ! -f /data/ipfs-cluster/service.json ]; then
		log "📰 no service.json found, creating one"
		ipfs-cluster-service init --consensus crdt
	fi

	log "🔍 reading hostname"
	PEER_HOSTNAME=$(cat /proc/sys/kernel/hostname)
	log "starting ipfs-cluster on ${PEER_HOSTNAME}"

	grep -q ".*-0$" /proc/sys/kernel/hostname
	if [ $? -eq 0 ]; then
		log "starting ipfs-cluster using the provided peer ID and private key"
		CLUSTER_ID=${BOOTSTRAP_PEER_ID} \
		CLUSTER_PRIVATEKEY=${BOOTSTRAP_PEER_PRIV_KEY} \
		exec ipfs-cluster-service daemon --upgrade
	else
		log "building the bootstrap address"
		BOOTSTRAP_ADDR=/dns4/${SVC_NAME}-0.${SVC_NAME}/tcp/9096/ipfs/${BOOTSTRAP_PEER_ID}
		if [ -z $BOOTSTRAP_ADDR ]; then
			log "no bootstrap address found, exiting"
			exit 1
		fi
		log "starting ipfs-cluster using the bootstrap address"
		# Only ipfs user can get here
		exec ipfs-cluster-service daemon --upgrade --bootstrap $BOOTSTRAP_ADDR --leave
	fi
}

print_container_info
for op in "$@"; do
	case $op in
		"debug")
		  log "💤 Sleeping indefinitely"
			sleep infinity
			log "✅ Done"
			;;
		"run")
			log "🏃 Running IPFS Cluster"
			run_ipfs_cluster
			log "✅ Done"
			;;
		*)
			log "😕 Operation '${op}' not defined"
			exit 1
			;;
	esac
done