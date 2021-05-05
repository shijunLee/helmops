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

type RepoType string

var (
	//+kubebuilder:validation:Enum=ChartMuseum,Git
	RepoTypeChartMuseum RepoType = "ChartMuseum"
	RepoTypeGit         RepoType = "Git"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// HelmRepoSpec defines the desired state of HelmRepo
type HelmRepoSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	//RepoName Chart repo name
	RepoName string `json:"repoName,omitempty"`
	//RepoType Chart repo type support git or chart museum
	RepoType RepoType `json:"repoType,omitempty"`

	//RepoURL chart repo url
	RepoURL string `json:"repoURL,omitempty"`

	//Username the user name for chart repo auth
	Username string `json:"username,omitempty"`

	//Password the user password for chart repo auth
	Password string `json:"password,omitempty"`

	//InsecureSkipTLS is skip tls verify
	InsecureSkipTLS bool `json:"insecureSkipTLS,omitempty"`

	//TLSSecretName if use tls get the tls secret name
	// <em>notice:</em> current not support
	TLSSecretName string `json:"tlsSecretName,omitempty"`
}

// HelmRepoStatus defines the observed state of HelmRepo
type HelmRepoStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Conditions []Condition `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:path=helmrepos
//+kubebuilder:resource:scope=Cluster
//+kubebuilder:printcolumn:name="Repo_Name",type="string",JSONPath=".spec.repoName"
//+kubebuilder:printcolumn:name="URL",type="string",JSONPath=".spec.repoURL"
//+kubebuilder:printcolumn:name="Type",type="string",JSONPath=".spec.repoType"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// HelmRepo is the Schema for the helmrepos API
type HelmRepo struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HelmRepoSpec   `json:"spec,omitempty"`
	Status HelmRepoStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// HelmRepoList contains a list of HelmRepo
type HelmRepoList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HelmRepo `json:"items"`
}

func init() {
	SchemeBuilder.Register(&HelmRepo{}, &HelmRepoList{})
}
