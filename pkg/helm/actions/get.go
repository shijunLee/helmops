package actions

import (
	"fmt"
	"strings"

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

func IsReleaseNotFound(err error) bool {
	return err.Error() == "release: not found"
}

//GetHelmFullOverrideName get helm release full overide name or full name for release object
func (i *GetOptions) GetHelmFullOverrideName() (string, error) {
	release, err := i.Run()
	if err != nil {
		return "", err
	}
	if release.Config != nil {
		fullOverrideName, ok := release.Config["fullnameOverride"]
		if !ok || fullOverrideName == "" {
			name := release.Chart.Metadata.Name
			overrideName, ok := release.Config["nameOverride"]
			if ok && overrideName != "" {
				overrideNameStr, ok := overrideName.(string)
				if ok {
					name = overrideNameStr
				}
			}
			if strings.Contains(release.Name, name) {
				returnName := release.Name
				if len(returnName) > 63 {
					returnName = string(returnName[:63])
				}
				returnName = strings.TrimSuffix(returnName, "-")
				return returnName, nil
			} else {
				returnName := fmt.Sprintf("%s-%s", release.Name, name)
				if len(returnName) > 63 {
					returnName = string(returnName[:63])
				}
				returnName = strings.TrimSuffix(returnName, "-")
				return returnName, nil
			}
		} else {
			fullOverrideNameStr, ok := fullOverrideName.(string)
			if ok {
				return fullOverrideNameStr, nil
			}
		}
	}
	return release.Name, nil
}
