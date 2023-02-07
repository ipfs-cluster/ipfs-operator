package controllers

import (
	"context"
	"fmt"

	clusterv1alpha1 "github.com/redhat-et/ipfs-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

func (r *IpfsClusterReconciler) EnsureCircuitRelay(
	ctx context.Context,
	m *clusterv1alpha1.IpfsCluster,
	secret *corev1.Secret,
) (err error) {
	log := ctrllog.FromContext(ctx)
	if err = r.createCircuitRelays(ctx, m, secret); err != nil {
		return fmt.Errorf("cannot create circuit relays: %w", err)
	}
	// Check the status of circuit relays.
	// wait for them to complte so we can determine announce addresses.
	for _, relayName := range m.Status.CircuitRelays {
		relay := clusterv1alpha1.CircuitRelay{}
		relay.Name = relayName
		relay.Namespace = m.Namespace
		if err = r.Client.Get(ctx, client.ObjectKeyFromObject(&relay), &relay); err != nil {
			return fmt.Errorf("could not lookup circuitRelay %q: %w", relayName, err)
		}
		if relay.Status.AddrInfo.ID == "" {
			log.Info("relay is not ready yet. Will continue waiting.", "relay", relayName)
			return fmt.Errorf("relay is not ready yet")
		}
	}
	if err = r.Status().Update(ctx, m); err != nil {
		return err
	}
	return nil
}

// createCircuitRelays Creates the necessary amount of circuit relays if any are missing.
// FIXME: if we change the number of CircuitRelays, we should update
// the IPFS config file as well.
func (r *IpfsClusterReconciler) createCircuitRelays(
	ctx context.Context,
	instance *clusterv1alpha1.IpfsCluster,
	secret *corev1.Secret,
) error {
	logger := log.FromContext(ctx, "context", "createCircuitRelays", "instance", instance)
	// do nothing
	if len(instance.Status.CircuitRelays) >= int(instance.Spec.Networking.CircuitRelays) {
		logger.Info("we have enough circuitRelays, skipping creation")
		// FIXME: handle scale-down of circuit relays
		return nil
	}
	logger.Info("creating more circuitRelays")
	// create the CircuitRelays
	for i := 0; int32(i) < instance.Spec.Networking.CircuitRelays; i++ {
		name := fmt.Sprintf("%s-%d", instance.Name, i)
		relay := clusterv1alpha1.CircuitRelay{}
		relay.Name = name
		relay.Namespace = instance.Namespace
		// include the private swarm key, if one is being provided
		if secret != nil {
			relay.Spec.SwarmKeyRef = &clusterv1alpha1.KeyRef{
				KeyName:    KeySwarmKey,
				SecretName: secret.Name,
			}
		}
		if err := ctrl.SetControllerReference(instance, &relay, r.Scheme); err != nil {
			return fmt.Errorf(
				"cannot set controller reference for new circuitRelay: %w, circuitRelay: %s",
				err, relay.Name,
			)
		}
		if err := r.Create(ctx, &relay); err != nil {
			return fmt.Errorf("cannot create new circuitRelay: %w", err)
		}
		instance.Status.CircuitRelays = append(instance.Status.CircuitRelays, relay.Name)
	}
	if err := r.Status().Update(ctx, instance); err != nil {
		return err
	}
	return nil
}
