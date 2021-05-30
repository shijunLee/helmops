/*
Copyright 2021 lishjun01@hotmail.com.

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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// HelmApplicationSpec defines the desired state of HelmApplication
type HelmApplicationSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// the install steps install
	Steps []ComponentStep `json:"steps"`
}

// HelmApplicationStatus defines the observed state of HelmApplication
type HelmApplicationStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// ComponentStep the step which will install
type ComponentStep struct {
	// the component will install
	ComponentName string `json:"componentName,omitempty"`
	// the component install params
	Parameters []ComponentParameter `json:"parameters,omitempty"`
}

// ComponentParameter the component install param ,notice not both set value and ref ,if set will use value as default
type ComponentParameter struct {
	// the parameter name
	Name string `json:"name,omitempty"`

	// the value for parameter ,notice not both set value and ref ,if set will use value as default
	Value string `json:"value,omitempty"`

	// the value type for parameter , will try to convert value to the set type not set will use default string
	Type string `json:"type,omitempty"`
	// ref parameter from other helm component
	Ref *RefParameter `json:"ref,omitempty"`
}

// RefParameter the parameter from the other component
type RefParameter struct {
	// the helm component Name
	ComponentName string `json:"componentName,omitempty"`
	// the helm component return date name define
	ReturnDateName string `json:"returnDateName,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// HelmApplication is the Schema for the helmapplications API
type HelmApplication struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HelmApplicationSpec   `json:"spec,omitempty"`
	Status HelmApplicationStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// HelmApplicationList contains a list of HelmApplication
type HelmApplicationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HelmApplication `json:"items"`
}

func init() {
	SchemeBuilder.Register(&HelmApplication{}, &HelmApplicationList{})
}
