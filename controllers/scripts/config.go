package scripts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"text/template"

	"github.com/alecthomas/units"
	"github.com/ipfs/kubo/config"
	"github.com/libp2p/go-libp2p-core/peer"
)

type configureIpfsOpts struct {
	FlattenedConfig string
}

const (
	configureIpfs = `
#!/bin/sh
set -e
set -x
user=ipfs
# This is a custom entrypoint for k8s designed to run ipfs nodes in an appropriate
# setup for production scenarios.

INDEX="${HOSTNAME##*-}"

PRIVATE_KEY=$(cat "/node-data/privateKey-${INDEX}")
PEER_ID=$(cat "/node-data/peerID-${INDEX}")

if [[ -f /data/ipfs/config ]]; then
	if [[ -f /data/ipfs/repo.lock ]]; then
		rm /data/ipfs/repo.lock
	fi
	exit 0
fi

echo '{{ .FlattenedConfig }}' > config.json
sed -i s,_peer-id_,"${PEER_ID}",g config.json
sed -i s,_private-key_,"${PRIVATE_KEY}",g config.json

ipfs init -- config.json

chown -R ipfs: /data/ipfs
`
	IPFSClusterEntrypoint = `
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
		CLUSTER_ID="${BOOTSTRAP_PEER_ID}" \
		CLUSTER_PRIVATEKEY="${BOOTSTRAP_PEER_PRIV_KEY}" \
		exec ipfs-cluster-service daemon --upgrade
	else
		log "building the bootstrap address"
		BOOTSTRAP_ADDR="/dns4/${SVC_NAME}-0.${SVC_NAME}/tcp/9096/ipfs/${BOOTSTRAP_PEER_ID}"
		if [ -z "${BOOTSTRAP_ADDR}" ]; then
			log "no bootstrap address found, exiting"
			exit 1
		fi
		log "starting ipfs-cluster using the bootstrap address"
		# Only ipfs user can get here
		exec ipfs-cluster-service daemon --upgrade --bootstrap "${BOOTSTRAP_ADDR}" --leave
	fi
}

for op in "${@}"; do
	case ${op} in
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
`
)

const (
	// BloomFalsePositiveRate Defines the probability of the bloom filter detecting a given value
	// as being stored.
	BloomFalsePositiveRate float64 = 0.001
	// BloomBlockSize Defines the size of blocks used by the bloom filter. This value is the denominator
	// when determining n = (total size) / (size of blocks).
	BloomBlockSize = 256 * units.Kibibyte
)

// CreateConfigureScript Accepts the given storageMax, peers, and relayClient
// and returns a completed configuration script which can be ran by the IPFS container config.
func CreateConfigureScript(
	storageMax string,
	peers []peer.AddrInfo,
	relayConfig config.RelayClient,
	bloomFilterSize int64,
) (string, error) {
	configureTmpl, _ := template.New("configureIpfs").Parse(configureIpfs)
	config, err := createTemplateConfig(storageMax, peers, relayConfig)
	if err != nil {
		return "", err
	}
	config.Swarm.RelayClient = relayConfig
	config.Datastore.BloomFilterSize = int(bloomFilterSize)
	configBytes, err := json.Marshal(config)
	if err != nil {
		return "", err
	}
	configureOpts := configureIpfsOpts{
		FlattenedConfig: string(configBytes),
	}
	configureBuf := new(bytes.Buffer)
	if err = configureTmpl.Execute(configureBuf, configureOpts); err != nil {
		return "", err
	}
	return configureBuf.String(), nil
}

// applyFlatfsServer Applies the given list of profiles to the kubo config object.
func applyFlatfsServer(conf *config.Config) error {
	for _, profile := range []string{"flatfs", "server"} {
		transformer, ok := config.Profiles[profile]
		if !ok {
			return fmt.Errorf("invalid configuration profile: %s", profile)
		}

		if err := transformer.Transform(conf); err != nil {
			return err
		}
	}
	return nil
}

// setFlatfsShardFunc Attempts to update the given flatfs configuration to use a shardFunc
// with the given `n`. If unsuccessful, false will be returned.
func setFlatfsShardFunc(conf *config.Config, n int8) error {
	// we want to use next-to-last/3 as the sharding function
	// as per this issue:
	// https://github.com/redhat-et/ipfs-operator/issues/32
	mounts, ok := conf.Datastore.Spec["mounts"].([]interface{})
	if !ok {
		return fmt.Errorf("could not convert mounts to interface")
	}

	// set /repo/flatfs/shard/v1/next-to-last/3 as the shardFunc for flatfs
	for _, mount := range mounts {
		var mountMap, child map[string]interface{}
		var dsType string
		mountMap, ok = mount.(map[string]interface{})
		if !ok {
			return fmt.Errorf("could not convert mount to m: string -> interface")
		}
		// get child
		child, ok = mountMap["child"].(map[string]interface{})
		if !ok {
			return fmt.Errorf("could not convert child to map string to interface")
		}
		// get type
		dsType, ok = child["type"].(string)
		if !ok {
			continue
		}
		if dsType == "flatfs" {
			child["shardFunc"] = "/repo/flatfs/shard/v1/next-to-last/" + strconv.FormatInt(int64(n), 10)
			break
		}
	}
	return nil
}

// applyIPFSClusterK8sDefaults Applies settings to the given Kubo configuration
// which are customized specifically for running within a Kubernetes cluster.
func applyIPFSClusterK8sDefaults(conf *config.Config, storageMax string, peers []peer.AddrInfo, rc config.RelayClient) {
	conf.Bootstrap = config.DefaultBootstrapAddresses
	conf.Addresses.API = config.Strings{"/ip4/0.0.0.0/tcp/5001"}
	conf.Addresses.Gateway = config.Strings{"/ip4/0.0.0.0/tcp/8080"}
	conf.Swarm.ConnMgr.HighWater = 2000
	conf.Datastore.BloomFilterSize = 1048576
	conf.Datastore.StorageMax = storageMax
	conf.Addresses.Swarm = []string{"/ip4/0.0.0.0/tcp/4001", "/ip6/::/tcp/4001"}
	conf.Swarm.EnableHolePunching = config.False
	conf.Swarm.RelayClient = rc
	conf.Peering.Peers = peers
	conf.Experimental.AcceleratedDHTClient = true
}

// createTemplateConfig Returns a kubo configuration which contains preconfigured
// settings that are optimized for running this within a Kubernetes cluster.
func createTemplateConfig(
	storageMax string,
	peers []peer.AddrInfo,
	rc config.RelayClient,
) (conf config.Config, err error) {
	// attempt to generate an identity

	// set keys + defaults
	conf.Identity.PeerID = "_peer-id_"
	conf.Identity.PrivKey = "_private-key_"

	// apply the server + flatfs profiles
	if err = applyFlatfsServer(&conf); err != nil {
		return
	}
	if err = setFlatfsShardFunc(&conf, 3); err != nil {
		return
	}
	applyIPFSClusterK8sDefaults(&conf, storageMax, peers, rc)
	return
}

// CalculateBloomFilterSize Accepts the size of the IPFS storage in bytes
// and returns the bloom filter size to be used in bytes.
//
// Computations are done with values based on this issue:
// https://github.com/redhat-et/ipfs-operator/issues/35#issue-1320941289
func CalculateBloomFilterSize(ipfsStorage int64) int64 {
	// formula based on bloom filter calculator:
	// https://hur.st/bloomfilter
	var m, p, k, r float64
	// false-negative rate, 1 / 1000
	p = BloomFalsePositiveRate
	// number of hash functions
	k = 7
	ipfsStorageAsBytes := units.Base2Bytes(ipfsStorage)

	n := ipfsStorageAsBytes / BloomBlockSize
	r = -k / math.Log(1-math.Exp(math.Log(p)/k))
	// number of blocks
	m = math.Ceil(float64(n) * r)
	// convert from bits -> bytes
	bloomFilterSizeBytes := int64(math.Ceil(m / 8))
	return bloomFilterSizeBytes
}
