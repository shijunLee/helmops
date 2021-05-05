package actions

import (
	"time"

	helmactions "helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/release"
)

type UninstallOptions struct {
	DisableHooks      bool
	DryRun            bool
	KeepHistory       bool
	Timeout           time.Duration
	Description       string
	ReleaseName       string
	KubernetesOptions *KubernetesClient
	Namespace         string
}

func (i *UninstallOptions) Run() (*release.UninstallReleaseResponse, error) {
	cfg, err := i.KubernetesOptions.GetHelmActionConfiguration(i.Namespace)
	if err != nil {
		return nil, err
	}
	uninstallConfig := helmactions.NewUninstall(cfg)
	uninstallConfig.DisableHooks = i.DisableHooks
	uninstallConfig.DryRun = i.DryRun
	uninstallConfig.KeepHistory = i.KeepHistory
	uninstallConfig.Timeout = i.Timeout
	uninstallConfig.Description = i.Description
	return uninstallConfig.Run(i.ReleaseName)
}
