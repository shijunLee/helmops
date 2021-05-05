package actions

import (
	"time"

	"github.com/pkg/errors"
	helmactions "helm.sh/helm/v3/pkg/action"
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
	return upgradeConfig.Run(i.ReleaseName, chartInfo, i.Values)
}
