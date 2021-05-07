package actions

import (
	"bytes"
	"context"
	"time"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/resource"

	"github.com/pkg/errors"
	helmactions "helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/postrender"
	"helm.sh/helm/v3/pkg/release"
)

type UpgradeOptions struct {
	// Install is a purely informative flag that indicates whether this upgrade was done in "install" mode.
	//
	// Applications may use this to determine whether this Upgrade operation was done as part of a
	// pure upgrade (Upgrade.Install == false) or as part of an install-or-upgrade operation
	// (Upgrade.Install == true).
	//
	// Setting this to `true` will NOT cause `Upgrade` to perform an install if the release does not exist.
	// That process must be handled by creating an Install action directly. See cmd/upgrade.go for an
	// example of how this flag is used.
	Install bool
	// Devel indicates that the operation is done in devel mode.
	Devel bool
	// Namespace is the namespace in which this operation should be performed.
	Namespace string
	// SkipCRDs skips installing CRDs when install flag is enabled during upgrade
	SkipCRDs bool
	// Timeout is the timeout for this operation
	Timeout time.Duration
	// Wait determines whether the wait operation should be performed after the upgrade is requested.
	Wait bool
	// DisableHooks disables hook processing if set to true.
	DisableHooks bool
	// DryRun controls whether the operation is prepared, but not executed.
	// If `true`, the upgrade is prepared but not performed.
	DryRun bool
	// Force will, if set to `true`, ignore certain warnings and perform the upgrade anyway.
	//
	// This should be used with caution.
	Force bool
	// ResetValues will reset the values to the chart's built-ins rather than merging with existing.
	ResetValues bool
	// ReuseValues will re-use the user's last supplied values.
	ReuseValues bool
	// Recreate will (if true) recreate pods after a rollback.
	Recreate bool
	// MaxHistory limits the maximum number of revisions saved per release
	MaxHistory int
	// Atomic, if true, will roll back on failure.
	Atomic bool
	// CleanupOnFail will, if true, cause the upgrade to delete newly-created resources on a failed update.
	CleanupOnFail bool
	// SubNotes determines whether sub-notes are rendered in the chart.
	SubNotes bool
	// Description is the description of this operation
	Description string
	// PostRender is an optional post-renderer
	//
	// If this is non-nil, then after templates are rendered, they will be sent to the
	// post renderer before sending to the Kuberntes API server.
	PostRenderer postrender.PostRenderer
	// DisableOpenAPIValidation controls whether OpenAPI validation is enforced.
	DisableOpenAPIValidation bool
	WaitForJobs              bool
	ChartOpts                *ChartOpts
	KubernetesOptions        *KubernetesClient
	Values                   map[string]interface{}
	ReleaseName              string
	// upgrade crd in upgrade actions
	UpgradeCRDs bool
}

func (i *UpgradeOptions) Run() (*release.Release, error) {
	cfg, err := i.KubernetesOptions.GetHelmActionConfiguration(i.Namespace)
	if err != nil {
		return nil, err
	}
	upgradeConfig := helmactions.NewUpgrade(cfg)
	upgradeConfig.SkipCRDs = i.SkipCRDs
	upgradeConfig.DryRun = i.DryRun
	upgradeConfig.RepoURL = i.ChartOpts.RepoOptions.RepoURL
	upgradeConfig.Timeout = i.Timeout
	upgradeConfig.DisableHooks = i.DisableHooks
	upgradeConfig.DisableOpenAPIValidation = i.DisableOpenAPIValidation
	upgradeConfig.Description = i.Description
	upgradeConfig.Namespace = i.Namespace
	upgradeConfig.Install = i.Install
	upgradeConfig.WaitForJobs = i.WaitForJobs
	upgradeConfig.CleanupOnFail = i.CleanupOnFail
	upgradeConfig.SubNotes = i.SubNotes
	upgradeConfig.Atomic = i.Atomic
	upgradeConfig.Wait = i.Wait
	upgradeConfig.MaxHistory = i.MaxHistory
	upgradeConfig.Recreate = i.Recreate
	upgradeConfig.ReuseValues = i.ReuseValues
	upgradeConfig.ResetValues = i.ResetValues
	upgradeConfig.Force = i.Force
	upgradeConfig.Namespace = i.KubernetesOptions.Namespace
	chartInfo, err := i.ChartOpts.LoadChart()
	if err != nil {
		return nil, errors.Wrapf(err, "load chart from config error")
	}
	if i.UpgradeCRDs {
		var crds = chartInfo.CRDObjects()
		err := updateCRDs(crds, cfg)
		if err != nil {
			return nil, err
		}
	}
	return upgradeConfig.Run(i.ReleaseName, chartInfo, i.Values)
}

func updateCRDs(crds []chart.CRD, cfg *helmactions.Configuration) error {
	totalItems := []*resource.Info{}
	for _, obj := range crds {
		// Read in the resources
		res, err := cfg.KubeClient.Build(bytes.NewBuffer(obj.File.Data), false)
		if err != nil {
			return errors.Wrapf(err, "failed to install CRD %s", obj.Name)
		}
		restConfig, err := cfg.RESTClientGetter.ToRESTConfig()
		if err != nil {
			return err
		}
		apiExtensionsClient, err := apiextensionsclient.NewForConfig(restConfig)
		if err != nil {
			return err
		}
		for _, item := range res {
			if item.Object.GetObjectKind().GroupVersionKind().Version == "v1" {
				obj, ok := item.Object.(*apiextensionsv1.CustomResourceDefinition)
				if !ok {
					cfg.Log("object not custom resource definition %s %s", item.Name, item.Namespace)
					continue
				}
				_, err := apiExtensionsClient.ApiextensionsV1().CustomResourceDefinitions().Get(context.Background(), obj.Name, metav1.GetOptions{})
				if err != nil {
					if k8serrors.IsNotFound(err) {
						_, err = apiExtensionsClient.ApiextensionsV1().CustomResourceDefinitions().Create(context.Background(),
							obj, metav1.CreateOptions{})
						if err != nil {
							return err
						}
						continue
					}
					return err
				}
				_, err = apiExtensionsClient.ApiextensionsV1().CustomResourceDefinitions().
					Update(context.Background(), obj, metav1.UpdateOptions{})
				if err != nil {
					return err
				}
			} else if item.Object.GetObjectKind().GroupVersionKind().Version == "v1beta1" {
				obj, ok := item.Object.(*apiextensionsv1beta1.CustomResourceDefinition)
				if !ok {
					cfg.Log("object not custom resource definition %s %s", item.Name, item.Namespace)
					continue
				}
				_, err := apiExtensionsClient.ApiextensionsV1beta1().CustomResourceDefinitions().Get(context.Background(),
					obj.Name, metav1.GetOptions{})
				if err != nil {
					if k8serrors.IsNotFound(err) {
						_, err = apiExtensionsClient.ApiextensionsV1beta1().CustomResourceDefinitions().Create(context.Background(),
							obj, metav1.CreateOptions{})
						if err != nil {
							return err
						}
						continue
					}
					return err
				}
				_, err = apiExtensionsClient.ApiextensionsV1beta1().CustomResourceDefinitions().
					Update(context.Background(), obj, metav1.UpdateOptions{})
				if err != nil {
					return err
				}
			}
		}
		totalItems = append(totalItems, res...)
	}
	if len(totalItems) > 0 {
		// Invalidate the local cache, since it will not have the new CRDs
		// present.
		discoveryClient, err := cfg.RESTClientGetter.ToDiscoveryClient()
		if err != nil {
			return err
		}
		cfg.Log("Clearing discovery cache")
		discoveryClient.Invalidate()
		// Give time for the CRD to be recognized.

		if err := cfg.KubeClient.Wait(totalItems, 60*time.Second); err != nil {
			return err
		}

		// Make sure to force a rebuild of the cache.
		discoveryClient.ServerGroups()
	}
	return nil
}
