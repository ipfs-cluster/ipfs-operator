package controllers

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/alecthomas/units"
	"github.com/ipfs/kubo/config"
	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
	clusterv1alpha1 "github.com/redhat-et/ipfs-operator/api/v1alpha1"
	"github.com/redhat-et/ipfs-operator/controllers/scripts"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	// ScriptConfigureIPFS Defines the script run by the IPFS containers
	// in order to initialize their state.
	ScriptConfigureIPFS = "configure-ipfs.sh"
	// ScriptIPFSClusterEntryPoint Defines a shell script used as the entrypoint
	// for the IPFS Cluster container.
	ScriptIPFSClusterEntryPoint = "entrypoint.sh"
)

// EnsureConfigMapScripts Returns a mutate function which loads the given configMap with scripts that
// customize the startup of the IPFS containers depending on the values from the given IPFS cluster resource.
func (r *IpfsClusterReconciler) EnsureConfigMapScripts(
	ctx context.Context,
	m *clusterv1alpha1.IpfsCluster,
	relayPeers []peer.AddrInfo,
	relayStatic []ma.Multiaddr,
	bootstrapPeers []string,
) (*corev1.ConfigMap, error) {
	var err error
	log := ctrllog.FromContext(ctx)
	cmName := "ipfs-cluster-scripts-" + m.Name
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmName,
			Namespace: m.Namespace,
		},
	}
	err = r.Get(ctx, client.ObjectKeyFromObject(cm), cm)
	if err != nil && !errors.IsNotFound(err) {
		return nil, fmt.Errorf("could not get configmap: %w", err)
	}

	op, err := ctrl.CreateOrUpdate(ctx, r.Client, cm, func() error {
		// TODO: compute these values in another function & place them here
		// convert multiaddrs to strings
		relayStaticStrs := make([]string, len(relayStatic))
		for i, maddr := range relayStatic {
			relayStaticStrs[i] = maddr.String()
		}

		relayConfig := config.RelayClient{
			Enabled:      config.True,
			StaticRelays: relayStaticStrs,
		}

		// compute storage sizes of IPFS volumes
		sizei64, ok := m.Spec.IpfsStorage.AsInt64()
		if !ok {
			sizei64 = m.Spec.IpfsStorage.ToDec().Value()
		}
		maxStorage := MaxIPFSStorage(sizei64)
		maxStorageS := fmt.Sprintf("%dB", maxStorage)
		bloomFilterSize := scripts.CalculateBloomFilterSize(maxStorage)

		// reprovider settings
		reproviderStrategy := m.Spec.Reprovider.Strategy
		if reproviderStrategy == "" {
			reproviderStrategy = clusterv1alpha1.ReproviderStrategyAll
		}
		reproviderInterval := m.Spec.Reprovider.Interval
		if reproviderInterval == "" {
			reproviderInterval = "12h"
		}

		// get the config script
		configScript, internalErr := scripts.CreateConfigureScript(
			maxStorageS,
			relayPeers,
			relayConfig,
			bloomFilterSize,
			reproviderInterval,
			string(reproviderStrategy),
			bootstrapPeers,
		)
		if internalErr != nil {
			return fmt.Errorf("could not create config script: %w", internalErr)
		}

		cm.Data = map[string]string{
			ScriptIPFSClusterEntryPoint: scripts.IPFSClusterEntrypoint,
			ScriptConfigureIPFS:         configScript,
		}
		if internalErr = ctrl.SetControllerReference(m, cm, r.Scheme); internalErr != nil {
			return fmt.Errorf("failed to set controller reference: %w", internalErr)
		}
		return nil
	})
	if err != nil {
		log.Error(err, "failed to createorupdate configmap", "operation", op, "configmap", cm)
		return nil, fmt.Errorf("could not create or update configmap: %w", err)
	}
	log.Info("completed createorupdate configmap", "operation", op, "configMap", cm)
	return cm, nil
}

// staticAddrsFromRelayPeers Extracts all of the static addresses from the
// given list of relayPeers.
func staticAddrsFromRelayPeers(relayPeers []peer.AddrInfo) ([]ma.Multiaddr, error) {
	relayStatic := make([]ma.Multiaddr, 0)
	for _, addrInfo := range relayPeers {
		p2ppart, err := ma.NewMultiaddr("/p2p/" + addrInfo.ID.String())
		if err != nil {
			return nil, fmt.Errorf("could not create p2p component: %w", err)
		}
		for _, addr := range addrInfo.Addrs {
			fullMa := addr.Encapsulate(p2ppart)
			relayStatic = append(relayStatic, fullMa)
		}
	}
	return relayStatic, nil
}

// getCircuitInfo Gets address info from the list of CircuitRelays
// and returns a list of AddrInfo.
func (r *IpfsClusterReconciler) getCircuitInfo(
	ctx context.Context,
	ipfs *clusterv1alpha1.IpfsCluster,
) ([]peer.AddrInfo, error) {
	log := ctrllog.FromContext(ctx)
	relayPeers := []peer.AddrInfo{}
	for _, relayName := range ipfs.Status.CircuitRelays {
		relay := clusterv1alpha1.CircuitRelay{}
		relay.Name = relayName
		relay.Namespace = ipfs.Namespace
		// OPTIMIZE: do this asynchronously?
		if err := r.Get(ctx, client.ObjectKeyFromObject(&relay), &relay); err != nil {
			return nil, fmt.Errorf("could not lookup circuitRelay: %w", err)
		}
		if err := relay.Status.AddrInfo.Parse(); err != nil {
			log.Error(err, "could not parse AddrInfo. Information will not be included in config", "relay", relayName)
			continue
		}
		addrInfo := relay.Status.AddrInfo.AddrInfo()
		relayPeers = append(relayPeers, *addrInfo)
	}
	return relayPeers, nil
}

// MaxIPSStorage Accepts a storage quantity and returns with a
// calculated value to be used for setting the Max IPFS storage value
// in bytes.
func MaxIPFSStorage(ipfsStorage int64) (storageMaxGB int64) {
	var reducedSize units.Base2Bytes
	// if the disk is big, use a bigger percentage of it.
	if units.Base2Bytes(ipfsStorage) > units.Tebibyte*8 {
		reducedSize = units.Base2Bytes(ipfsStorage) * 9 / 10
	} else {
		reducedSize = units.Base2Bytes(ipfsStorage) * 8 / 10
	}
	return int64(reducedSize)
}
