/*
Copyright 2025.

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

type Thresholds struct {
	CPUUsageNano     int64           `json:"cpuUsageNano,omitempty"`     // CPU usage in nanoseconds (e.g. 200_000_000)
	MemoryUsageBytes int64           `json:"memoryUsageBytes,omitempty"` // Memory usage in bytes (e.g. 200 * 1024 * 1024)
	MaxRestartCount  int32           `json:"maxRestartCount,omitempty"`  // Maximum allowed restarts before considering unhealthy (e.g. 10)
	MaxPendingTime   metav1.Duration `json:"maxPendingTime,omitempty"`   // Maximum allowed pending time before considering unhealthy (e.g. 60 seconds)
}

// WorkloadSpec defines the desired state of Workload.
type WorkloadSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	JobName    string     `json:"jobName"`
	Thresholds Thresholds `json:"thresholds,omitempty"` // Thresholds for evaluating workload health
	// Important: Run "make" to regenerate code after modifying this file
}

// WorkloadStatus defines the observed state of Workload.
type WorkloadStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Evaluated bool `json:"evaluated"`
	Health    bool `json:"health"` // Health status of the workload, true if healthy, false otherwise
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Workload is the Schema for the workloads API.
type Workload struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WorkloadSpec   `json:"spec,omitempty"`
	Status WorkloadStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// WorkloadList contains a list of Workload.
type WorkloadList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Workload `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Workload{}, &WorkloadList{})
}
