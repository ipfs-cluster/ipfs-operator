package controllers

import (
	"context"

	"github.com/libp2p/go-libp2p/core/peer"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/redhat-et/ipfs-operator/api/v1alpha1"
)

// EnsureRelayCircuitInfo Returns information about the configured CircuitRelays.
func (r *IpfsClusterReconciler) EnsureRelayCircuitInfo(ctx context.Context, ipfs *v1alpha1.IpfsCluster) (
	relayPeers []peer.AddrInfo, relayStatic []ma.Multiaddr, err error) {
	if relayPeers, err = r.getCircuitInfo(ctx, ipfs); err != nil {
		return
	}
	if relayStatic, err = staticAddrsFromRelayPeers(relayPeers); err != nil {
		return
	}
	return
}
