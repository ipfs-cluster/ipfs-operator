package scripts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"strconv"
	"strings"

	"github.com/ipfs/interface-go-ipfs-core/options"
	"github.com/ipfs/kubo/config"
	"github.com/libp2p/go-libp2p-core/peer"
)

type configureIpfsOpts struct {
	FlattenedConfig string
}

const (
	// TODO: dockerize kubo and move this script to the container
	configureIpfs = `
#!/bin/sh
set -e
set -x
user=ipfs
# This is a custom entrypoint for k8s designed to run ipfs nodes in an appropriate
# setup for production scenarios.

if [[ -f /data/ipfs/config ]]; then
	if [[ -f /data/ipfs/repo.lock ]]; then
		rm /data/ipfs/repo.lock
	fi
	exit 0
fi

echo '{{ .FlattenedConfig }}' > config.json
ipfs init -- config.json
MYSELF=$(ipfs id -f="<id>")

# use 'next-to-last/3' as the sharding function
sed 's/next-to-last\/2/next-to-last\/3/g' /data/ipfs/config

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

// CreateConfigureScript Accepts the given storageMax, peers, and relayClient
// and returns a completed configuration script which can be ran by the IPFS container config.
func CreateConfigureScript(storageMax string, peers []peer.AddrInfo, relayConfig config.RelayClient) (string, error) {
	configureTmpl, _ := template.New("configureIpfs").Parse(configureIpfs)
	config, err := createTemplateConfig(storageMax, peers, relayConfig)
	config.Swarm.RelayClient = relayConfig
	if err != nil {
		return "", err
	}
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

// applyProfiles Applies the given list of profiles to the kubo config object.
func applyProfiles(conf *config.Config, profiles string) error {
	if profiles == "" {
		return nil
	}

	for _, profile := range strings.Split(profiles, ",") {
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
func setFlatfsShardFunc(conf *config.Config, n uint8) error {
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
			child["shardFunc"] = "/repo/flatfs/shard/v1/next-to-last/" + strconv.FormatUint(uint64(n), 10)
			break
		}
	}
	return nil
}

// applyIPFSClusterK8sDefaults Applies settings to the given Kubo configuration
// which are customized specifically for running within a Kubernetes cluster.
func applyIPFSClusterK8sDefaults(conf *config.Config, storageMax string, peers []peer.AddrInfo, rc config.RelayClient) {
	conf.Addresses.API = config.Strings{"/ip4/0.0.0.0/tcp/5001"}
	conf.Addresses.Gateway = config.Strings{"/ip4/0.0.0.0/tcp/8080"}
	conf.Swarm.ConnMgr.HighWater = 2000
	conf.Datastore.BloomFilterSize = 1048576
	conf.Datastore.StorageMax = storageMax
	conf.Swarm.EnableHolePunching = config.True

	conf.Swarm.RelayClient = rc
	conf.Peering.Peers = peers
}

// createTemplateConfig Returns a kubo configuration which contains preconfigured
// settings that are optimized for running this within a Kubernetes cluster.
func createTemplateConfig(
	storageMax string,
	peers []peer.AddrInfo,
	rc config.RelayClient,
) (conf config.Config, err error) {
	// attempt to generate an identity
	var identity config.Identity

	// try to generate an elliptic curve key first
	identity, err = config.CreateIdentity(os.Stdout, []options.KeyGenerateOption{
		options.Key.Type(options.Ed25519Key),
	})
	// fall back to RSA
	if err != nil {
		identity, err = config.CreateIdentity(os.Stdout, []options.KeyGenerateOption{
			options.Key.Type(options.RSAKey),
			options.Key.Size(4096),
		})
		if err != nil {
			return
		}
	}

	// set keys + defaults
	conf.Identity = identity

	// apply the server + flatfs profiles
	if err = applyProfiles(&conf, "flatfs,server"); err != nil {
		return
	}
	if err = setFlatfsShardFunc(&conf, 3); err != nil {
		return
	}
	applyIPFSClusterK8sDefaults(&conf, storageMax, peers, rc)
	return
}
