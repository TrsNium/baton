/*

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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// BatonSpec defines the desired state of Baton
type BatonSpec struct {
	Deployment        `json:"deployment"`
	Strategies        []Strategy `json:"strategies"`
	IntervalSec       int32      `json:"intervalSec"`
	MonitorTimeoutSec int32      `json:"monitorTimeoutSec"`
}

type Deployment struct {
	Name      string `json:"name"`
	NameSpace string `json:"namespace"`
}

// BatonStatus defines the observed state of Baton
type BatonStatus struct {
	LastRunStartedAt    string `json:"last_run_started_at"`
	LastSuccessfulRunAt string `json:"last_successful_run_at"`
}

// +kubebuilder:object:root=true

// Baton is the Schema for the batons API
type Baton struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BatonSpec   `json:"spec,omitempty"`
	Status BatonStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// BatonList contains a list of Baton
type BatonList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Baton `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Baton{}, &BatonList{})
}
