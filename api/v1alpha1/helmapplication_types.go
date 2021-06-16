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

	// current install step name use ComponentReleaseName for default
	CurrentStepName string `json:"currentStepName,omitempty"`

	//the install conditions
	Conditions []Condition `json:"conditions,omitempty"`

	//the return data for install steps
	StepReturns []StepReturn `json:"stepReturns,omitempty"`

	// the application install status
	Status string `json:"status,omitempty"`
}

//StepReturn the application install step return values which define in componse
type StepReturn struct {
	StepName string `json:"stepName,omitempty"`

	Values []CreateParam `json:"values,omitempty"`
}

//StepStatus the application install step status
type StepStatus struct {
	// the step name use for ComponentReleaseName
	Name string `json:"stepName,omitempty"`

	// the step process status
	Status string `json:"status,omitempty"`
}

// ComponentStep the step which will install
type ComponentStep struct {
	// the component will install
	ComponentName string `json:"componentName,omitempty"`

	// the component release name ,if not set will use the appname-chartname
	ComponentReleaseName string `json:"componentReleaseName,omitempty"`

	// notice this will check helm template resource , if have resource install ,
	// this will not install and only install or update the old object,
	// only support clusterrole serviceaccount and other global resource
	CheckIfExistNotInstall bool `json:"checkIfExistNotInstall,omitempty"`

	// the component install params
	Parameters CreateParam `json:"parameters,omitempty"`

	// the values ref from before component release
	ValuesRefComponentRelease []string `json:"ValuesRefComponentRelease,omitempty"`
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
