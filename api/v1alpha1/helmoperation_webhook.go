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
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var helmoperationlog = logf.Log.WithName("helmoperation-resource")

func (r *HelmOperation) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

//+kubebuilder:webhook:path=/mutate-helmops-shijunlee-net-v1alpha1-helmoperation,mutating=true,failurePolicy=fail,sideEffects=None,groups=helmops.shijunlee.net,resources=helmoperations,verbs=create;update,versions=v1alpha1,name=mhelmoperation.kb.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Defaulter = &HelmOperation{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *HelmOperation) Default() {
	helmoperationlog.Info("default", "name", r.Name)

	// TODO(user): fill in your defaulting logic.
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-helmops-shijunlee-net-v1alpha1-helmoperation,mutating=false,failurePolicy=fail,sideEffects=None,groups=helmops.shijunlee.net,resources=helmoperations,verbs=create;update,versions=v1alpha1,name=vhelmoperation.kb.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Validator = &HelmOperation{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *HelmOperation) ValidateCreate() error {
	helmoperationlog.Info("validate create", "name", r.Name)
	if r.Spec.ChartVersion == "" || r.Spec.ChartName == "" {
		return errors.New("chart name or chart version can not empty")
	}
	// TODO(user): fill in your validation logic upon object creation.
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *HelmOperation) ValidateUpdate(old runtime.Object) error {
	helmoperationlog.Info("validate update", "name", r.Name)
	oldOperation, ok := old.(*HelmOperation)
	if !ok {
		return nil
	}
	if r.Spec.ChartName != oldOperation.Spec.ChartName || r.Spec.ChartRepoName != oldOperation.Spec.ChartRepoName {
		return errors.New("chart name or chart repo can not change for update")
	}
	// TODO(user): fill in your validation logic upon object update.
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *HelmOperation) ValidateDelete() error {
	helmoperationlog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}
