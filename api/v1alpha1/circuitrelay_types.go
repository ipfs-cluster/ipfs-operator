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

type CircuitRelaySpec struct {
}

type CircuitRelayStatus struct {
	PeerID        string   `json:"peerid"`
	AnnounceAddrs []string `json:"announceaddrs"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// CircuitRelay is the Schema for the circuitrelays API
type CircuitRelay struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CircuitRelaySpec   `json:"spec,omitempty"`
	Status CircuitRelayStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// CircuitRelayList contains a list of CircuitRelay
type CircuitRelayList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CircuitRelay `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CircuitRelay{}, &CircuitRelayList{})
}
