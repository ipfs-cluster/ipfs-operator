package controllers

import (
	"context"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/ipfs/kubo/config"
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
	clusterv1alpha1 "github.com/redhat-et/ipfs-operator/api/v1alpha1"
	"github.com/redhat-et/ipfs-operator/controllers/scripts"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

// configMapScripts Returns a mutate function which loads the given configMap with scripts that
// customize the startup of the IPFS containers depending on the values from the given IPFS cluster resource.
func (r *IpfsReconciler) configMapScripts(
	ctx context.Context,
	m *clusterv1alpha1.IpfsCluster,
	cm *corev1.ConfigMap,
) (controllerutil.MutateFn, string) {
	log := ctrllog.FromContext(ctx)
	relayPeers := []peer.AddrInfo{}
	relayStatic := []string{}
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
		relayPeers = append(relayPeers, *ai)
		p2ppart, err := ma.NewMultiaddr("/p2p/" + ai.ID.String())
		if err != nil {
			log.Error(err, "could not create p2p component during configMapScripts", "relay", relayName)
		}
		for _, addr := range ai.Addrs {
			fullMa := addr.Encapsulate(p2ppart).String()
			relayStatic = append(relayStatic, fullMa)
		}
	}

	cmName := "ipfs-cluster-scripts-" + m.Name
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

	relayConfig := config.RelayClient{
		Enabled:      config.True,
		StaticRelays: relayStatic,
	}

	// get the config script
	configScript, err := scripts.CreateConfigureScript(
		storageMaxGB,
		relayPeers,
		relayConfig,
	)
	if err != nil {
		return func() error {
			return err
		}, ""
	}

	expected := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmName,
			Namespace: m.Namespace,
		},
		Data: map[string]string{
			"entrypoint.sh":     scripts.IPFSClusterEntrypoint,
			"configure-ipfs.sh": configScript,
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
