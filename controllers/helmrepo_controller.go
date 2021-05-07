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
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/shijunLee/helmops/pkg/helm/actions"
	"helm.sh/helm/v3/pkg/storage/driver"
	"k8s.io/client-go/rest"

	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"k8s.io/apimachinery/pkg/util/wait"

	"k8s.io/client-go/util/workqueue"

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
	Log                     logr.Logger
	Scheme                  *runtime.Scheme
	Period                  int
	LocalCachePath          string
	queue                   workqueue.RateLimitingInterface
	MaxConcurrentReconciles int
	JitterPeriod            time.Duration
	RestConfig              *rest.Config
}

type syncUpdateHelmRelease struct {
	ReleaseName  string
	Namespace    string
	ChartName    string
	ChartVersion string
	ChartRepo    string
}

var (
	repoCache = &sync.Map{}
)

func NewHelmRepoReconciler(mgr ctrl.Manager, period, maxConcurrentReconciles int, jitterPeriod time.Duration, localCachePath string) *HelmRepoReconciler {
	if maxConcurrentReconciles == 0 {
		maxConcurrentReconciles = 1
	}
	result := &HelmRepoReconciler{
		Client:                  mgr.GetClient(),
		Log:                     mgr.GetLogger(),
		Scheme:                  mgr.GetScheme(),
		Period:                  period,
		RestConfig:              mgr.GetConfig(),
		LocalCachePath:          localCachePath,
		MaxConcurrentReconciles: maxConcurrentReconciles,
		JitterPeriod:            jitterPeriod,
	}
	queue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "repo-job-queue")
	result.queue = queue
	return result
}

func (r *HelmRepoReconciler) startUpdateProcess(ctx context.Context) {
	if r.JitterPeriod == 0 {
		r.JitterPeriod = 1 * time.Second
	}
	for i := 0; i < r.MaxConcurrentReconciles; i++ {
		go wait.UntilWithContext(ctx, func(ctx context.Context) {
			// Run a worker thread that just dequeues items, processes them, and marks them done.
			// It enforces that the reconcileHandler is never invoked concurrently with the same object.
			for r.processNextWorkItem(ctx) {
			}
		}, r.JitterPeriod)
	}
}

func (r *HelmRepoReconciler) processNextWorkItem(ctx context.Context) bool {
	obj, shutdown := r.queue.Get()
	if shutdown {
		// Stop working
		return false
	}

	// We call Done here so the workqueue knows we have finished
	// processing this item. We also must remember to call Forget if we
	// do not want this work item being re-queued. For example, we do
	// not call Forget if a transient error occurs, instead the item is
	// put back on the workqueue and attempted again after a back-off
	// period.
	defer r.queue.Done(obj)
	r.processHelmReleaseUpgradeRreconcileHandler(ctx, obj)
	return true
}

func (r *HelmRepoReconciler) processHelmReleaseUpgradeRreconcileHandler(ctx context.Context, obj interface{}) {
	// Make sure that the the object is a valid request.
	req, ok := obj.(syncUpdateHelmRelease)
	if !ok {
		// As the item in the workqueue is actually invalid, we call
		// Forget here else we'd go into a loop of attempting to
		// process a work item that is invalid.
		r.queue.Forget(obj)
		r.Log.Error(nil, "Queue item was not a Request", "type", fmt.Sprintf("%T", obj), "value", obj)
		// Return true, don't take a break
		return
	}

	log := r.Log.WithValues("releaseName", req.ReleaseName, "namespace", req.Namespace)
	ctx = logf.IntoContext(ctx, log)

	// RunInformersAndControllers the syncHandler, passing it the namespace/Name string of the
	// resource to be synced.
	if result, err := r.DoRepoSyncReconcile(ctx, req); err != nil {
		r.queue.AddRateLimited(req)
		log.Error(err, "Reconciler error")
		return
	} else if result.RequeueAfter > 0 {
		// The result.RequeueAfter request will be lost, if it is returned
		// along with a non-nil error. But this is intended as
		// We need to drive to stable reconcile loops before queuing due
		// to result.RequestAfter
		r.queue.Forget(obj)
		r.queue.AddAfter(req, result.RequeueAfter)
		return
	} else if result.Requeue {
		r.queue.AddRateLimited(req)
		return
	}

	// Finally, if no error occurs we Forget this item so it does not
	// get queued again until another change happens.
	r.queue.Forget(obj)

}

func (r *HelmRepoReconciler) DoRepoSyncReconcile(ctx context.Context, req syncUpdateHelmRelease) (ctrl.Result, error) {
	helmOperation := &helmopsv1alpha1.HelmOperation{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: req.ReleaseName, Namespace: req.Namespace}, helmOperation)
	if err != nil {
		r.Log.Error(err, "find helm operation resource from client error", "ResourceName", req.ReleaseName, "ResourceName", req.Namespace)
		return ctrl.Result{}, err
	}
	// if is delete do nothing return
	if !helmOperation.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	var getOptions = actions.GetOptions{
		ReleaseName:       helmOperation.Name,
		Namespace:         helmOperation.Namespace,
		KubernetesOptions: actions.NewKubernetesClient(actions.WithRestConfig(r.RestConfig)),
	}

	release, err := getOptions.Run()
	if err != nil {
		if err == driver.ErrReleaseNotFound {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{RequeueAfter: 5 * time.Second}, err
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
	if !chartRepo.Operation.CheckChartExist(helmOperation.Spec.ChartName, req.ChartVersion) {
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
	chartOptions := &actions.ChartOpts{
		ChartName:    req.ChartName,
		ChartVersion: req.ChartVersion,
	}
	switch pathType {
	case "file":
		chartOptions.LocalPath = url
	case "http":
		chartOptions.ChartURL = url
	}

	var values = release.Config
	var installChartVersion = release.Chart.Metadata.Version
	// if version change or value changes do update process
	if installChartVersion == req.ChartVersion {
		if helmOperation.Status.CurrentChartVersion != installChartVersion {
			helmOperation.Status.CurrentChartVersion = installChartVersion
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
	if helmOperation.Spec.ChartVersion != req.ChartVersion &&
		helmOperation.Status.CurrentChartVersion != req.ChartVersion &&
		installChartVersion != req.ChartVersion {
		updateConfig := helmOperation.Spec.Upgrade
		chart := *chartOptions
		chart.ChartVersion = req.ChartVersion
		updateOption := actions.UpgradeOptions{
			Values:                   values,
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
		}
		release, err = updateOption.Run()
		if err != nil {
			r.Log.Error(err, "upgrade release user helm client error")
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
	return ctrl.Result{}, nil
}

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
	if err != nil {
		r.Log.Error(err, "repo sybc call back return error")
	}
	var operationList = &helmopsv1alpha1.HelmOperationList{}
	err = r.List(context.Background(), operationList)
	if err != nil {
		r.Log.Error(err, "list helm operation error")
		return
	}
	for _, item := range operationList.Items {
		if item.Spec.ChartRepoName == chart.RepoName &&
			item.Spec.ChartName == chart.Name &&
			item.Spec.ChartVersion != chart.Version {
			r.queue.Add(syncUpdateHelmRelease{
				Namespace:    item.Namespace,
				ChartName:    chart.Name,
				ChartRepo:    chart.RepoName,
				ChartVersion: chart.Version,
				ReleaseName:  item.Name,
			})
		}
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *HelmRepoReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&helmopsv1alpha1.HelmRepo{}).
		Complete(r)
}

//removeFinalizer Stop the job ,do not delete installed helm release
func (r *HelmRepoReconciler) removeFinalizer(ctx context.Context, helmRepo *helmopsv1alpha1.HelmRepo) error {
	repoInfo, ok := repoCache.Load(helmRepo.Name)
	if !ok {
		return nil
	}
	chartRepo, ok := repoInfo.(*charts.ChartRepo)
	if !ok {
		// if repo not found ,do not process this operation
		//todo update status for this helmOperation
		return errors.New("convert repo item to cache")
	}
	chartRepo.Close()
	return nil
}
