package chartmuseum

import (
	"github.com/pkg/errors"
	"github.com/shijunLee/helmops/pkg/helm/actions"
	"github.com/shijunLee/helmops/pkg/helm/utils"
	"helm.sh/helm/v3/pkg/repo"
)

var (
	ChartNotExistErr        = errors.New("chart not exit for chartMuseum")
	ChartVersionNotExistErr = errors.New("chart version not exit for chartMuseum")
)

type ChartMuseum struct {
	URL             string
	Username        string
	Password        string
	RepoName        string
	InsecureSkipTLS bool
}

func NewChartMuseum(url, username, password, repoName string, insecureSkipTLS bool) (*ChartMuseum, error) {
	c := &ChartMuseum{
		URL:             url,
		Username:        username,
		Password:        password,
		RepoName:        repoName,
		InsecureSkipTLS: insecureSkipTLS,
	}
	_, err := c.loadIndex()
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (c *ChartMuseum) loadIndex() (map[string]repo.ChartVersions, error) {
	repoOptions := actions.RepoOptions{
		RepoURL:               c.URL,
		Username:              c.Username,
		Password:              c.Password,
		InsecureSkipTLSVerify: c.InsecureSkipTLS,
		RepoName:              c.RepoName,
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

func (c *ChartMuseum) GetChartVersionUrl(chartName, chartVersion string) (url, pathType string, err error) {
	chartVersions, err := c.loadIndex()
	if err != nil {
		return "", "", err
	}
	if versions, ok := chartVersions[chartName]; ok {
		for _, item := range versions {
			if item.Version == chartVersion {
				return item.URLs[0], "http", nil
			}
		}
	} else {
		return "", "", ChartNotExistErr
	}
	return "", "", ChartVersionNotExistErr
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

func (c *ChartMuseum) ListCharts() (map[string]utils.CommonChartVersions, error) {
	index, err := c.loadIndex()
	if err != nil {
		return nil, err
	}
	var result = map[string]utils.CommonChartVersions{}
	for key, versions := range index {
		for _, item := range versions {
			commonCharts, ok := result[key]
			if ok {
				commonCharts = append(commonCharts, utils.CommonChartVersion{Name: key, Version: item.Version, URLType: "http", URL: item.URLs[0]})
				result[key] = commonCharts
			} else {
				result[key] = utils.CommonChartVersions{{Name: key, Version: item.Version, URLType: "http", URL: item.URLs[0]}}
			}
		}
	}
	return result, nil
}
