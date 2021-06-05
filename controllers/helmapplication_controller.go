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

package controllers

import (
	"context"

	"github.com/go-logr/logr"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	helmopsv1alpha1 "github.com/shijunLee/helmops/api/v1alpha1"
)

const (
	helmApplicationFinalizer = "finalizer.helmapplication.helmops.shijunlee.net"
)

// HelmApplicationReconciler reconciles a HelmApplication object
type HelmApplicationReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=helmops.shijunlee.net,resources=helmapplications,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=helmops.shijunlee.net,resources=helmapplications/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=helmops.shijunlee.net,resources=helmapplications/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the HelmApplication object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.7.2/pkg/reconcile
func (r *HelmApplicationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("helmapplication", req.NamespacedName)

	helmApplication := &helmopsv1alpha1.HelmApplication{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: req.Name, Namespace: req.Namespace}, helmApplication)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "find helm application resource from client error", "ResourceName", req.Name, "ResourceName", req.Namespace)
		return ctrl.Result{}, err
	}
	if !helmApplication.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(helmApplication, helmApplicationFinalizer) {
			controllerutil.AddFinalizer(helmApplication, helmApplicationFinalizer)
			err = r.Client.Update(ctx, helmApplication)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		if err = r.removeFinalizer(ctx, helmApplication); err != nil {
			return ctrl.Result{}, err
		}
		controllerutil.RemoveFinalizer(helmApplication, helmRepoFinalizer)
	}

	// your logic here

	return ctrl.Result{}, nil
}

func (r *HelmApplicationReconciler) removeFinalizer(ctx context.Context, helmApplication *helmopsv1alpha1.HelmApplication) error {
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *HelmApplicationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&helmopsv1alpha1.HelmApplication{}).
		Complete(r)
}

// buildStepReleaseHelmOperation build install step releaseHelmOperation for application,the values is the before step return values
func buildStepReleaseHelmOperation(helmComponent *helmopsv1alpha1.HelmComponent,values map[string]interface{}) (*helmopsv1alpha1.HelmOperation, error) {

	return nil, nil
}

// watchStepReleaseReady get the step release is ready
func watchStepReleaseReady(operation *helmopsv1alpha1.HelmOperation, helmComponent *helmopsv1alpha1.HelmComponent) (bool, error) {
	return false, nil
}

// get step release return data
func getStepReleaseReturnData(operation *helmopsv1alpha1.HelmOperation, helmComponent *helmopsv1alpha1.HelmComponent) (map[string]interface{}, error) {
	return nil, nil
}
