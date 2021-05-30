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

// HelmComponentSpec defines the desired state of HelmComponent
type HelmComponentSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	//+kubebuilder:pruning:PreserveUnknownFields
	//Values the helm install values , if values update while update the helm release
	ValuesTemplate ComponentTemplate `json:"valuesTemplate,omitempty"`
	//AutoUpdate is auto update for release
	AutoUpdate bool `json:"autoUpdate,omitempty"`

	//ChartRepoName the helmops repo name
	ChartRepoName string `json:"chartRepoName,omitempty"`
	//ChartVersion the version for the chart will install
	ChartVersion string `json:"chartVersion,omitempty"`
	//ChartName the chart name which will install
	ChartName string `json:"chartName,omitempty"`
	// Create the chart create options
	Create Create `json:"create,omitempty"`

	//Upgrade the chart upgrade options
	Upgrade Upgrade `json:"upgrade,omitempty"`

	//Uninstall the chart uninstall options
	Uninstall Uninstall `json:"uninstall,omitempty"`

	// to get the helm release is running , if not set will not wait for this job
	StableStatus *StableStatus `json:"stableStatus,omitempty"`

	// the component return value for  next helm component
	ReturnValues []ReturnValue `json:"ReturnValues,omitempty"`
}

// HelmComponentStatus defines the observed state of HelmComponent
type HelmComponentStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//ComponentTemplate the component Value template
type ComponentTemplate struct {
	CUE  *CUETemplate    `json:"cue,omitempty"`
	YAML *GoYAMLTemplate `json:"yaml,omitempty"`
}

//CUETemplate the cue template for helm values in helm template
type CUETemplate struct {
	Template string `json:"template,omitempty"`
}

// GoYAMLTemplate go yaml template for helm template
type GoYAMLTemplate struct {
	Template *CreateParam `json:"template,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// HelmComponent is the Schema for the helmcomponents API
type HelmComponent struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HelmComponentSpec   `json:"spec,omitempty"`
	Status HelmComponentStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// HelmComponentList contains a list of HelmComponent
type HelmComponentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HelmComponent `json:"items"`
}

// StableStatus this is to get helm component stable status
type StableStatus struct {
	APIGroup string `json:"apiGroup,omitempty"`
	Version  string `json:"version,omitempty"`
	Resource string `json:"resource,omitempty"`
	JSONPath string `json:"jsonPath,omitempty"`
	Value    string `json:"value,omitempty"`
}

type ReturnValue struct {
	Name          string   `json:"name"`
	APIGroup      string   `json:"apiGroup,omitempty"`
	Version       string   `json:"version,omitempty"`
	Resource      string   `json:"resource,omitempty"`
	JSONPaths     []string `json:"jsonPaths,omitempty"`
	ValueTemplate string   `json:"valueTemplate,omitempty"`
}

func init() {
	SchemeBuilder.Register(&HelmComponent{}, &HelmComponentList{})
}
