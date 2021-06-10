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
	"reflect"
	"time"

	"github.com/go-logr/logr"
	"helm.sh/helm/v3/pkg/storage/driver"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	helmopsv1alpha1 "github.com/shijunLee/helmops/api/v1alpha1"
	"github.com/shijunLee/helmops/pkg/charts"
	"github.com/shijunLee/helmops/pkg/helm/actions"
	"github.com/shijunLee/helmops/pkg/helm/utils"
)

const (
	helmOperationFinalizer = "finalizer.helmoperation.helmops.shijunlee.net"
)

// HelmOperationReconciler reconciles a HelmOperation object
type HelmOperationReconciler struct {
	client.Client
	Log        logr.Logger
	Scheme     *runtime.Scheme
	RestConfig *rest.Config
}

//+kubebuilder:rbac:groups=helmops.shijunlee.net,resources=helmoperations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=helmops.shijunlee.net,resources=helmoperations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=helmops.shijunlee.net,resources=helmoperations/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// the HelmOperation object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.

func (r *HelmOperationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("helmoperation", req.NamespacedName)

	helmOperation := &helmopsv1alpha1.HelmOperation{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: req.Name, Namespace: req.Namespace}, helmOperation)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "find helm operation resource from client error", "ResourceName", req.Name, "ResourceName", req.Namespace)
		return ctrl.Result{}, err
	}
	if helmOperation.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(helmOperation, helmOperationFinalizer) {
			controllerutil.AddFinalizer(helmOperation, helmOperationFinalizer)
			err = r.Client.Update(ctx, helmOperation)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		if err = r.removeFinalizer(ctx, helmOperation); err != nil {
			return ctrl.Result{}, err
		}
		controllerutil.RemoveFinalizer(helmOperation, helmOperationFinalizer)
	}
	var getOptions = actions.GetOptions{
		ReleaseName:       helmOperation.Name,
		Namespace:         helmOperation.Namespace,
		KubernetesOptions: actions.NewKubernetesClient(actions.WithRestConfig(r.RestConfig)),
	}
	var notCreate = false
	release, err := getOptions.Run()
	if err != nil {
		if err == driver.ErrReleaseNotFound {
			notCreate = true
		}
	}
	chartOptions := &actions.ChartOpts{
		ChartName:    helmOperation.Spec.ChartName,
		ChartVersion: helmOperation.Spec.ChartVersion,
	}
	repoInfo, ok := repoCache.Load(helmOperation.Spec.ChartRepoName)
	if !ok {
		// if repo not found ,do not process this operation
		//todo update status for this helmOperation
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	chartRepo, ok := repoInfo.(*charts.ChartRepo)
	if !ok {
		// if repo not found ,do not process this operation
		//todo update status for this helmOperation
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}
	if !chartRepo.Operation.CheckChartExist(helmOperation.Spec.ChartName, helmOperation.Spec.ChartVersion) {
		// if repo not found ,do not process this operation
		//todo update status for this helmOperation , the chart version not exist
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}
	url, pathType, err := chartRepo.Operation.GetChartVersionUrl(helmOperation.Spec.ChartName, helmOperation.Spec.ChartVersion)
	if err != nil {
		// if repo not found ,do not process this operation
		//todo update status for this helmOperation , the chart version not exist
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}
	switch pathType {
	case "file":
		chartOptions.LocalPath = url
	case "http":
		chartOptions.ChartURL = url
	}

	// if release not create  do create
	if notCreate {
		var createInfo = helmOperation.Spec.Create
		installOptions := actions.InstallOptions{
			KubernetesOptions:        actions.NewKubernetesClient(actions.WithRestConfig(r.RestConfig)),
			ReleaseName:              helmOperation.Name,
			Namespace:                helmOperation.Namespace,
			CreateNamespace:          helmOperation.Spec.Create.CreateNamespace,
			ChartOpts:                chartOptions,
			Description:              createInfo.Description,
			SkipCRDs:                 createInfo.SkipCRDs,
			Timeout:                  createInfo.Timeout,
			NoHook:                   createInfo.NoHook,
			GenerateName:             createInfo.GenerateName,
			DisableOpenAPIValidation: createInfo.DisableOpenAPIValidation,
			IsUpgrade:                createInfo.IsUpgrade,
			WaitForJobs:              createInfo.WaitForJobs,
			Replace:                  createInfo.Replace,
			Wait:                     createInfo.Wait,
			Values:                   helmOperation.Spec.Values.Object,
		}
		release, err = installOptions.Run()
		if err != nil {
			log.Error(err, "install release user helm client error")
			return ctrl.Result{RequeueAfter: 10 * time.Second}, err
		}
		helmOperation.Status.CurrentChartVersion = release.Chart.Metadata.Version
		helmOperation.Status.ReleaseStatus = string(release.Info.Status)
		err = r.Client.Status().Update(ctx, helmOperation)
		if err != nil {
			// if repo not found ,do not process this operation
			//todo update status for this helmOperation , the chart version not exist
			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		}
	} else {
		var values = release.Config
		var installChartVersion = release.Chart.Metadata.Version
		// if version change or value changes do update process
		if (helmOperation.Spec.ChartVersion != installChartVersion && helmOperation.Status.CurrentChartVersion != installChartVersion) ||
			!reflect.DeepEqual(values, helmOperation.Spec.Values.Object) {
			// if the installed helm release chart version great the the operation, update operation version and return
			if utils.GetVersionGreaterThan(installChartVersion, helmOperation.Status.CurrentChartVersion) {
				helmOperation.Status.CurrentChartVersion = installChartVersion
				if reflect.DeepEqual(values, helmOperation.Spec.Values.Object) {
					helmOperation.Status.ReleaseStatus = string(release.Info.Status)
					err = r.Client.Status().Update(ctx, helmOperation)
					if err != nil {
						// if repo not found ,do not process this operation
						//todo update status for this helmOperation , the chart version not exist
						return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
					}
					return ctrl.Result{}, nil
				}
			}
			updateConfig := helmOperation.Spec.Upgrade
			chart := *chartOptions
			if utils.GetVersionGreaterThan(helmOperation.Status.CurrentChartVersion, helmOperation.Spec.ChartVersion) {
				chart.ChartVersion = helmOperation.Status.CurrentChartVersion
			}
			updateOption := actions.UpgradeOptions{
				Values:                   helmOperation.Spec.Values.Object,
				Install:                  updateConfig.Install,
				Devel:                    updateConfig.Devel,
				Namespace:                helmOperation.Namespace,
				SkipCRDs:                 updateConfig.SkipCRDs,
				Timeout:                  updateConfig.Timeout,
				Wait:                     updateConfig.Wait,
				DisableHooks:             updateConfig.DisableHooks,
				Force:                    updateConfig.Force,
				ResetValues:              updateConfig.ResetValues,
				ReuseValues:              updateConfig.ReuseValues,
				Recreate:                 updateConfig.Recreate,
				MaxHistory:               updateConfig.MaxHistory,
				Atomic:                   updateConfig.Atomic,
				CleanupOnFail:            updateConfig.CleanupOnFail,
				SubNotes:                 updateConfig.SubNotes,
				Description:              updateConfig.Description,
				DisableOpenAPIValidation: updateConfig.DisableOpenAPIValidation,
				WaitForJobs:              updateConfig.WaitForJobs,
				ChartOpts:                &chart,
				KubernetesOptions:        actions.NewKubernetesClient(actions.WithRestConfig(r.RestConfig)),
				UpgradeCRDs:              updateConfig.UpgradeCRDs,
			}
			release, err = updateOption.Run()
			if err != nil {
				log.Error(err, "upgrade release user helm client error")
				return ctrl.Result{RequeueAfter: 10 * time.Second}, err
			}
			helmOperation.Status.CurrentChartVersion = release.Chart.Metadata.Version
			helmOperation.Status.ReleaseStatus = string(release.Info.Status)
			err = r.Client.Status().Update(ctx, helmOperation)
			if err != nil {
				// if repo not found ,do not process this operation
				//todo update status for this helmOperation , the chart version not exist
				return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
			}
		}
	}

	return ctrl.Result{}, nil
}
func (r *HelmOperationReconciler) removeFinalizer(ctx context.Context, operation *helmopsv1alpha1.HelmOperation) error {
	if operation.Spec.Uninstall.DoNotDeleteRelease {
		return nil
	}
	uninstallConfig := operation.Spec.Uninstall
	uninstall := actions.UninstallOptions{
		Description:       uninstallConfig.Description,
		KeepHistory:       uninstallConfig.KeepHistory,
		Timeout:           uninstallConfig.Timeout,
		DisableHooks:      uninstallConfig.DisableHooks,
		Namespace:         operation.Namespace,
		ReleaseName:       operation.Name,
		KubernetesOptions: actions.NewKubernetesClient(actions.WithRestConfig(r.RestConfig)),
	}
	_, err := uninstall.Run()
	if err != nil {
		return err
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *HelmOperationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&helmopsv1alpha1.HelmOperation{}).
		Complete(r)
}
