package chartmuseum

import (
	"github.com/pkg/errors"
	"github.com/shijunLee/helmops/pkg/helm/actions"
	"github.com/shijunLee/helmops/pkg/helm/utils"
	"helm.sh/helm/v3/pkg/repo"
)

var (
	ChartNotExistErr = errors.New("chart not exit for chartMuseum")
)

type ChartMuseum struct {
	URL                string
	Username           string
	Password           string
	InsecureVerifySkip bool
}

func (c *ChartMuseum) loadIndex() (map[string]repo.ChartVersions, error) {
	repoOptions := actions.RepoOptions{
		RepoURL:               c.URL,
		Username:              c.Username,
		Password:              c.Password,
		InsecureSkipTLSVerify: c.InsecureVerifySkip,
	}
	repoIndex, err := repoOptions.GetLatestRepoIndex()
	if err != nil {
		return nil, err
	}
	return repoIndex.Entries, nil
}

func (c *ChartMuseum) GetChartLastVersion(chartName string) (string, error) {
	vers, err := c.getChartVersions(chartName)
	if err != nil {
		return "", err
	}
	return utils.GetLatestSemver(vers)
}

func (c *ChartMuseum) GetChartVersionUrl() (url, pathType string, err error) {

	return "", "", err
}

func (c *ChartMuseum) CheckChartExist(chartName, version string) bool {
	vers, err := c.getChartVersions(chartName)
	if err != nil {
		return false
	}
	for _, item := range vers {
		if item == version {
			return true
		}
	}
	return false
}

func (c *ChartMuseum) getChartVersions(chartName string) ([]string, error) {
	chartVersions, err := c.loadIndex()
	if err != nil {
		return nil, err
	}
	versions, ok := chartVersions[chartName]
	if !ok {
		return nil, ChartNotExistErr
	}
	var vers []string
	for _, item := range versions {
		vers = append(vers, item.Version)
	}
	return vers, nil
}
