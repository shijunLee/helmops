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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// HelmOperationSpec defines the desired state of HelmOperation
type HelmOperationSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	//+kubebuilder:pruning:PreserveUnknownFields
	//Values the helm install values , if values update while update the helm release
	Values CreateParam `json:"values,omitempty"`
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
}

type Upgrade struct {
	//Install Setting this to `true` will NOT cause `Upgrade` to perform an install if the release does not exist.
	// That process must be handled by creating an Install action directly. See cmd/upgrade.go for an
	// example of how this flag is used.
	Install bool `json:"install,omitempty"`

	// Devel indicates that the operation is done in devel mode.
	Devel bool `json:"devel,omitempty"`

	// SkipCRDs skips installing CRDs when install flag is enabled during upgrade
	SkipCRDs bool `json:"skipCRDs,omitempty"`

	// Timeout is the timeout for this operation
	Timeout time.Duration `json:"timeout,omitempty"`
	// Wait determines whether the wait operation should be performed after the upgrade is requested.
	Wait bool `json:"wait,omitempty"`
	// DisableHooks disables hook processing if set to true.
	DisableHooks bool `json:"disableHooks,omitempty"`

	// Force will, if set to `true`, ignore certain warnings and perform the upgrade anyway.
	//
	// This should be used with caution.
	Force bool `json:"force,omitempty"`
	// ResetValues will reset the values to the chart's built-ins rather than merging with existing.
	ResetValues bool `json:"resetValues,omitempty"`
	// ReuseValues will re-use the user's last supplied values.
	ReuseValues bool `json:"reuseValues,omitempty"`
	// Recreate will (if true) recreate pods after a rollback.
	Recreate bool `json:"recreate,omitempty"`

	// MaxHistory limits the maximum number of revisions saved per release
	MaxHistory int `json:"maxHistory,omitempty"`

	// Atomic, if true, will roll back on failure.
	Atomic bool `json:"atomic,omitempty"`

	// CleanupOnFail will, if true, cause the upgrade to delete newly-created resources on a failed update.
	CleanupOnFail bool `json:"cleanupOnFail,omitempty"`

	// SubNotes determines whether sub-notes are rendered in the chart.
	SubNotes bool `json:"subNotes,omitempty"`

	// Description is the description of this operation
	Description string `json:"description,omitempty"`

	// DisableOpenAPIValidation controls whether OpenAPI validation is enforced.
	DisableOpenAPIValidation bool `json:"disableOpenAPIValidation,omitempty"`
	//WaitForJobs wait for jobs exec success
	WaitForJobs bool `json:"waitForJobs,omitempty"`

	// is upgrade CRD when upgrade the helm release
	UpgradeCRDs bool `json:"UpgradeCRDs,omitempty"`
}

//Create the helm chart create options
type Create struct {
	// Description install custom description
	Description string `json:"description,omitempty"`
	// SkipCRDs is skip crd when install
	SkipCRDs bool `json:"skipCRDs,omitempty"`
	// Timeout is the timeout for this operation
	Timeout time.Duration `json:"timeout,omitempty"`
	// NoHook do not use hook
	NoHook bool `json:"noHook,omitempty"`
	//GenerateName auto generate name for a release
	GenerateName bool `json:"generateName,omitempty"`
	//CreateNamespace create namespace when install
	CreateNamespace bool `json:"createNamespace,omitempty"`
	//DisableOpenAPIValidation disable openapi validation on kubernetes install
	DisableOpenAPIValidation bool `json:"disableOpenAPIValidation,omitempty"`
	// IsUpgrade is upgrade dependence charts
	IsUpgrade bool `json:"isUpgrade,omitempty"`

	// WaitForJobs wait job exec success
	WaitForJobs bool `json:"waitForJobs,omitempty"`

	//Replace  while resource exist do replace operation
	Replace bool `json:"replace,omitempty"`

	//Wait wait  runtime.Object is running
	Wait bool `json:"wait,omitempty"`
}

type Uninstall struct {
	// DisableHooks disables hook processing if set to true.
	DisableHooks bool `json:"disableHooks,omitempty"`
	// KeepHistory keep chart install history
	KeepHistory bool `json:"keepHistory,omitempty"`
	// TimeOut time out time
	Timeout time.Duration `json:"timeout,omitempty"`
	// Description install custom description
	Description string `json:"description,omitempty"`
	// do not delete helm release if helm operation is delete
	DoNotDeleteRelease bool `json:"doNotDeleteRelease,omitempty"`
}

// HelmOperationStatus defines the observed state of HelmOperation
type HelmOperationStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	LastUpdateTime      *metav1.Time `json:"updateTime,omitempty"`
	Conditions          []Condition  `json:"conditions,omitempty"`
	CurrentChartVersion string       `json:"currentChartVersion,omitempty"`
	ReleaseStatus       string       `json:"releaseStatus"`
}

type Condition struct {
	LastTransitionTime *metav1.Time `json:"lastTransitionTime,omitempty"`
	Message            string       `json:"message,omitempty"`
	Reason             string       `json:"reason,omitempty"`
	Status             string       `json:"status,omitempty"`
	Type               string       `json:"type,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="ChartName",type="string",JSONPath=".spec.chartName"
//+kubebuilder:printcolumn:name="ChartVersion",type="string",JSONPath=".spec.chartVersion"
//+kubebuilder:printcolumn:name="RepoName",type="string",JSONPath=".spec.chartRepoName"
//+kubebuilder:printcolumn:name="AutoUpdate",type="bool",JSONPath=".spec.autoUpdate"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// HelmOperation is the Schema for the helmoperations API
type HelmOperation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HelmOperationSpec   `json:"spec,omitempty"`
	Status HelmOperationStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// HelmOperationList contains a list of HelmOperation
type HelmOperationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HelmOperation `json:"items"`
}

func init() {
	SchemeBuilder.Register(&HelmOperation{}, &HelmOperationList{})
}
