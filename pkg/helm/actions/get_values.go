package actions

import helmactions "helm.sh/helm/v3/pkg/action"

type GetValuesOptions struct {
	Version           int
	ReleaseName       string
	Namespace         string
	KubernetesOptions *KubernetesClient
	AllValues         bool
}

//Run do get value method
func (i *GetValuesOptions) Run() (map[string]interface{}, error) {
	cfg, err := i.KubernetesOptions.GetHelmActionConfiguration(i.Namespace)
	if err != nil {
		return nil, err
	}
	getValuesConfig := helmactions.NewGetValues(cfg)
	getValuesConfig.AllValues = i.AllValues
	getValuesConfig.Version = i.Version
	return getValuesConfig.Run(i.ReleaseName)
}
