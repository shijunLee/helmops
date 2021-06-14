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
	helmopsv1alpha1 "github.com/shijunLee/helmops/api/v1alpha1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	helmComponentFinalizer = "finalizer.helmcomponent.helmops.shijunlee.net"
)

// HelmComponentReconciler reconciles a HelmComponent object
type HelmComponentReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=helmops.shijunlee.net,resources=helmcomponents,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=helmops.shijunlee.net,resources=helmcomponents/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=helmops.shijunlee.net,resources=helmcomponents/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *HelmComponentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := r.Log.WithValues("helmcomponent", req.NamespacedName)

	helmComponent := &helmopsv1alpha1.HelmComponent{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: req.Name, Namespace: req.Namespace}, helmComponent)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			l.Info("helm component not found")
			return ctrl.Result{}, nil
		}
		l.Error(err, "find helm component resource from client error", "ResourceName", req.Name, "ResourceName", req.Namespace)
		return ctrl.Result{}, err
	}
	if !helmComponent.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(helmComponent, helmComponentFinalizer) {
			controllerutil.AddFinalizer(helmComponent, helmComponentFinalizer)
			err = r.Client.Update(ctx, helmComponent)
			if err != nil {
				l.Error(err, "update  helm component error for add finalizer")
				return ctrl.Result{}, err
			}
		}
	} else {
		if err = r.removeFinalizer(ctx, helmComponent); err != nil {
			l.Error(err, "remove finalizer for helm component error")
			return ctrl.Result{}, err
		}
		controllerutil.RemoveFinalizer(helmComponent, helmRepoFinalizer)
		err = r.Client.Update(ctx, helmComponent)
		if err != nil {
			l.Error(err, "update helm component error")
			return ctrl.Result{}, err
		}
	}
	// your logic here

	return ctrl.Result{}, nil
}

// do some system process like system delete process
func (r *HelmComponentReconciler) removeFinalizer(ctx context.Context, operation *helmopsv1alpha1.HelmComponent) error {
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *HelmComponentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&helmopsv1alpha1.HelmComponent{}).
		Complete(r)
}
