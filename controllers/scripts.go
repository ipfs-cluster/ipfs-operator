package controllers

import (
	"bytes"
	"context"
	encjson "encoding/json"
	"strconv"
	tmpl "text/template"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/json"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
	clusterv1alpha1 "github.com/redhat-et/ipfs-operator/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

type configureIpfsOpts struct {
	StorageMax      string
	RelayClientJSON string
	PeersJSON       string
}

var (
	// TODO: dockerize kubo and move this script to the container
	configureIpfs = `
#!/bin/sh
set -e
set -x
user=ipfs
# This is a custom entrypoint for k8s designed to run ipfs nodes in an appropriate
# setup for production scenarios.

if [ -f /data/ipfs/config ]; then
	if [ -f /data/ipfs/repo.lock ]; then
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

// configMapScripts Returns a mutate function which loads the given configMap with scripts that
// customize the startup of the IPFS containers depending on the values from the given IPFS cluster resource.
func (r *IpfsReconciler) configMapScripts(
	ctx context.Context,
	m *clusterv1alpha1.Ipfs,
	cm *corev1.ConfigMap,
) (controllerutil.MutateFn, string) {
	log := ctrllog.FromContext(ctx)
	relayPeers := []*peer.AddrInfo{}
	relayStatic := []*ma.Multiaddr{}
	for _, relayName := range m.Status.CircuitRelays {
		relay := clusterv1alpha1.CircuitRelay{}
		relay.Name = relayName
		relay.Namespace = m.Namespace
		err := r.Get(ctx, client.ObjectKeyFromObject(&relay), &relay)
		if err != nil {
			log.Error(err, "could not lookup circuitRelay during confgMapScripts", "relay", relayName)
			return nil, ""
		}
		if err = relay.Status.AddrInfo.Parse(); err != nil {
			log.Error(err, "could not parse AddrInfo. Information will not be included in config", "relay", relayName)
			continue
		}
		ai := relay.Status.AddrInfo.AddrInfo()
		relayPeers = append(relayPeers, ai)
		p2ppart, err := ma.NewMultiaddr("/p2p/" + ai.ID.String())
		if err != nil {
			log.Error(err, "could not create p2p component during configMapScripts", "relay", relayName)
		}
		for _, addr := range ai.Addrs {
			fullMa := addr.Encapsulate(p2ppart)
			relayStatic = append(relayStatic, &fullMa)
		}
	}

	cmName := "ipfs-cluster-scripts-" + m.Name
	configureTmpl, _ := tmpl.New("configureIpfs").Parse(configureIpfs)
	var storageMaxGB string
	parsed, err := resource.ParseQuantity(m.Spec.IpfsStorage)
	if err != nil {
		storageMaxGB = "100"
	} else {
		sizei64, _ := parsed.AsInt64()
		sizeGB := sizei64 / 1024 / 1024 / 1024
		var reducedSize int64
		// if the disk is big, use a bigger percentage of it.
		if sizeGB > 1024*8 {
			reducedSize = sizeGB * 9 / 10
		} else {
			reducedSize = sizeGB * 8 / 10
		}
		storageMaxGB = strconv.Itoa(int(reducedSize))
	}

	relayClientConfig := map[string]interface{}{
		"Enabled":      true,
		"StaticRelays": relayStatic,
	}
	relayClientConfigJSON, _ := json.Marshal(relayClientConfig)
	peeringConfigJSON, _ := encjson.Marshal(relayPeers)

	configureOpts := configureIpfsOpts{
		StorageMax:      storageMaxGB,
		PeersJSON:       string(peeringConfigJSON),
		RelayClientJSON: string(relayClientConfigJSON),
	}
	configureBuf := new(bytes.Buffer)
	_ = configureTmpl.Execute(configureBuf, configureOpts)

	expected := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmName,
			Namespace: m.Namespace,
		},
		Data: map[string]string{
			"configure-ipfs.sh": configureBuf.String(),
		},
	}
	expected.DeepCopyInto(cm)
	if err = ctrl.SetControllerReference(m, cm, r.Scheme); err != nil {
		return func() error {
			return err
		}, ""
	}
	return func() error {
		return nil
	}, cmName
}
