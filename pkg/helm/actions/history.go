package actions

import (
	helmactions "helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/release"
)

type HistoryOptions struct {
	Max               int
	Version           int
	Namespace         string
	KubernetesOptions *KubernetesClient
	ReleaseName       string
}

func (i *HistoryOptions) Run() ([]*release.Release, error) {
	cfg, err := i.KubernetesOptions.GetHelmActionConfiguration(i.Namespace)
	if err != nil {
		return nil, err
	}
	historyConfig := helmactions.NewHistory(cfg)
	historyConfig.Version = i.Version
	historyConfig.Max = i.Max
	return historyConfig.Run(i.ReleaseName)
}
