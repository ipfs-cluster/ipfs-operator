/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// ConditionReconciled is a status condition type that indicates whether the
	// CR has been successfully reconciled
	ConditionReconciled string = "Reconciled"
	// ReconciledReasonComplete indicates the CR was successfully reconciled
	ReconciledReasonComplete string = "ReconcileComplete"
	// ReconciledReasonError indicates an error was encountered while
	// reconciling the CR
	ReconciledReasonError string = "ReconcileError"
)

type IpfsSpec struct {
	URL            string `json:"url"`
	Public         bool   `json:"public"`
	IpfsStorage    string `json:"ipfsStorage"`
	ClusterStorage string `json:"clusterStorage"`
	Replicas       int32  `json:"replicas"`
	CircuitRelays  int32  `json:"circuitRelays"`
}

type IpfsStatus struct {
	Conditions    []metav1.Condition `json:"conditions,omitempty"`
	CircuitRelays []string           `json:"circuitRelays,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Ipfs is the Schema for the ipfs API
type Ipfs struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IpfsSpec   `json:"spec,omitempty"`
	Status IpfsStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// IpfsList contains a list of Ipfs
type IpfsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Ipfs `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Ipfs{}, &IpfsList{})
}
