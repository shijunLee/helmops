package actions

import (
	helmactions "helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/release"
)

type GetOptions struct {
	Version           int
	ReleaseName       string
	Namespace         string
	KubernetesOptions *KubernetesClient
}

func (i *GetOptions) Run() (*release.Release, error) {
	cfg, err := i.KubernetesOptions.GetHelmActionConfiguration(i.Namespace)
	if err != nil {
		return nil, err
	}
	getConfig := helmactions.NewGet(cfg)
	getConfig.Version = i.Version
	return getConfig.Run(i.ReleaseName)
}
