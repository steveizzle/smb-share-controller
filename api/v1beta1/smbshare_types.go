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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// SmbShareSpec defines the desired state of SmbShare
type SmbShareSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Path to did share
	Path string `json:"path"`
	// Secret Reference Name
	SecretName string `json:"secretName"`
	// MountOptions with default values
	// +kubebuilder:default={"file_mode=0700", "dir_mode=0777", "uid=1001", "gid=1001", "vers=3.0"}
	MountOptions []string `json:"mountOptions,omitempty"`
}

// SmbShareStatus defines the observed state of SmbShare
type SmbShareStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	PvName  string `json:"pvname"`
	PvcName string `json:"pvcname"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="PVName",type=string,JSONPath=`.status.pvname`
//+kubebuilder:printcolumn:name="PVCName",type=string,JSONPath=`.status.pvcname`
//+kubebuilder:printcolumn:name="Path",type=string,JSONPath=`.spec.path`

// SmbShare is the Schema for the smbshares API
// +kubebuilder:subresource:status
type SmbShare struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SmbShareSpec   `json:"spec,omitempty"`
	Status SmbShareStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SmbShareList contains a list of SmbShare
type SmbShareList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SmbShare `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SmbShare{}, &SmbShareList{})
}
