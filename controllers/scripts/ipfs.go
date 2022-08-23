package scripts

import (
	"bytes"
	"html/template"
)

type configureIpfsOpts struct {
	StorageMax      string
	RelayClientJSON string
	PeersJSON       string
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

ipfs init --profile=badgerds,server
MYSELF=$(ipfs id -f="<id>")

ipfs config Addresses.API /ip4/0.0.0.0/tcp/5001
ipfs config Addresses.Gateway /ip4/0.0.0.0/tcp/8080
ipfs config --json Swarm.ConnMgr.HighWater 2000
ipfs config --json Datastore.BloomFilterSize 1048576
ipfs config Datastore.StorageMax {{ .StorageMax }}GB
ipfs config --json Swarm.RelayClient '{{ .RelayClientJSON }}'
ipfs config --json Swarm.EnableHolePunching true
ipfs config --json Peering.Peers '{{ .PeersJSON }}'
ipfs config Datastore.StorageMax 100GB

chown -R ipfs: /data/ipfs
`
)

// CreateConfigureScript Accepts the given storageMax, peersJSON, and relayClientJSON
// and returns a completed configuration script which can be ran by the IPFS container config.
func CreateConfigureScript(storageMax, peersJSON, relayClientJSON string) (string, error) {
	configureTmpl, _ := template.New("configureIpfs").Parse(configureIpfs)
	configureOpts := configureIpfsOpts{
		StorageMax:      storageMax,
		PeersJSON:       peersJSON,
		RelayClientJSON: relayClientJSON,
	}
	configureBuf := new(bytes.Buffer)
	if err := configureTmpl.Execute(configureBuf, configureOpts); err != nil {
		return "", err
	}
	return configureBuf.String(), nil
}
