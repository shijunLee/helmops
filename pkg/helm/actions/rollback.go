package actions

import (
	"time"

	helmactions "helm.sh/helm/v3/pkg/action"
)

type RollBackOptions struct {
	Namespace         string
	KubernetesOptions *KubernetesClient
	ReleaseName       string

	Version       int
	Timeout       time.Duration
	Wait          bool
	DisableHooks  bool
	DryRun        bool
	Recreate      bool // will (if true) recreate pods after a rollback.
	Force         bool // will (if true) force resource upgrade through uninstall/recreate if needed
	CleanupOnFail bool
}

func (i *RollBackOptions) Run() error {
	cfg, err := i.KubernetesOptions.GetHelmActionConfiguration(i.Namespace)
	if err != nil {
		return err
	}
	rollBackConfig := helmactions.NewRollback(cfg)
	rollBackConfig.Version = i.Version
	rollBackConfig.Timeout = i.Timeout
	rollBackConfig.Wait = i.Wait
	rollBackConfig.DisableHooks = i.DisableHooks
	rollBackConfig.DryRun = i.DryRun
	rollBackConfig.Recreate = i.Recreate
	rollBackConfig.Force = i.Force
	rollBackConfig.CleanupOnFail = i.CleanupOnFail
	return rollBackConfig.Run(i.ReleaseName)

}
