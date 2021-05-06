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
	"sync"
	"time"

	"github.com/shijunLee/helmops/pkg/helm/utils"

	"github.com/shijunLee/helmops/pkg/charts"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"k8s.io/apimachinery/pkg/types"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	helmopsv1alpha1 "github.com/shijunLee/helmops/api/v1alpha1"
)

const (
	helmRepoFinalizer = "finalizer.helmrepo.helmops.shijunlee.net"
)

// HelmRepoReconciler reconciles a HelmRepo object
type HelmRepoReconciler struct {
	client.Client
	Log            logr.Logger
	Scheme         *runtime.Scheme
	Period         int
	LocalCachePath string
}

var (
	repoCache = &sync.Map{}
)

//+kubebuilder:rbac:groups=helmops.shijunlee.net,resources=helmrepos,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=helmops.shijunlee.net,resources=helmrepos/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=helmops.shijunlee.net,resources=helmrepos/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the HelmRepo object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.7.2/pkg/reconcile
func (r *HelmRepoReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("helmrepo", req.NamespacedName)
	log.Info("Watch repo reconciler event ", "ResourceName", req.Name)
	// your logic here
	helmRepo := &helmopsv1alpha1.HelmRepo{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: req.Name}, helmRepo)
	if err != nil {
		log.Error(err, "find repo resource from client error", "ResourceName", req.Name)
		return ctrl.Result{}, err
	}
	if !helmRepo.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(helmRepo, helmRepoFinalizer) {
			controllerutil.AddFinalizer(helmRepo, helmRepoFinalizer)
			err = r.Client.Update(ctx, helmRepo)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		if err = r.removeFinalizer(ctx, helmRepo); err != nil {
			return ctrl.Result{}, err
		}
		controllerutil.RemoveFinalizer(helmRepo, helmRepoFinalizer)
	}
	repo, err := charts.NewChartRepo(helmRepo.Name,
		string(helmRepo.Spec.RepoType), helmRepo.Spec.RepoURL, helmRepo.Spec.Username,
		helmRepo.Spec.Password, helmRepo.Spec.GitAuthToken, helmRepo.Spec.GitBranch,
		r.LocalCachePath, helmRepo.Spec.InsecureSkipTLS, r.Period)
	if err != nil {
		log.Error(err, "create repo error", "ResourceName", req.Name)
		return ctrl.Result{RequeueAfter: time.Second * 5}, err
	}
	repo.StartTimerJobs(r.repoCallBack)
	repoCache.Store(helmRepo.Name, repo)

	return ctrl.Result{}, nil
}

//repoCallBack repo period event call back , return chart and version , get witch you need to process
func (r *HelmRepoReconciler) repoCallBack(chart *utils.CommonChartVersion, err error) {

}

// SetupWithManager sets up the controller with the Manager.
func (r *HelmRepoReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&helmopsv1alpha1.HelmRepo{}).
		Complete(r)
}

func (r *HelmRepoReconciler) removeFinalizer(ctx context.Context, helmRepo *helmopsv1alpha1.HelmRepo) error {
	return nil
}
