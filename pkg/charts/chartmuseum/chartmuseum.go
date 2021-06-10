package chartmuseum

import (
	"fmt"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"helm.sh/helm/v3/pkg/repo"

	"github.com/shijunLee/helmops/pkg/helm/actions"
	"github.com/shijunLee/helmops/pkg/helm/utils"
	"github.com/shijunLee/helmops/pkg/log"
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
	repoOptions     *actions.RepoOptions
}

func NewChartMuseum(url, username, password, repoName string, insecureSkipTLS bool) (*ChartMuseum, error) {
	c := &ChartMuseum{
		URL:             url,
		Username:        username,
		Password:        password,
		RepoName:        repoName,
		InsecureSkipTLS: insecureSkipTLS,
	}
	repoOptions := &actions.RepoOptions{
		RepoURL:               c.URL,
		Username:              c.Username,
		Password:              c.Password,
		InsecureSkipTLSVerify: c.InsecureSkipTLS,
		RepoName:              c.RepoName,
	}
	c.repoOptions = repoOptions
	err := c.addLocalRepo()
	if err != nil {
		log.GlobalLog.WithName("chartmuseum").Error(err, "add local repo error")
		return nil, err
	}
	_, err = c.loadIndex()
	if err != nil {
		log.GlobalLog.WithName("chartmuseum").Error(err, "load repo index error")
		return nil, err
	}
	return c, nil
}

func (c *ChartMuseum) addLocalRepo() error {
	return c.repoOptions.AddChartRepoToLocal()
}

func (c *ChartMuseum) loadIndex() (map[string]repo.ChartVersions, error) {

	repoIndex, err := c.repoOptions.GetLatestRepoIndex()
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
				var chartURL = item.URLs[0]
				if strings.HasPrefix(strings.ToLower(chartURL), "http") {
					return chartURL, "http", nil
				}
				chartURL = fmt.Sprintf("%s/%s", c.repoOptions.RepoURL, chartURL)
				return chartURL, "http", nil
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
	// if update fail not process
	//TODO: log for helm local cache update fail
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = c.repoOptions.UpdateRepoLocalCache()
	}()
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
	wg.Wait()
	return vers, nil
}

func (c *ChartMuseum) ListCharts() (map[string]utils.CommonChartVersions, error) {
	// if update fail not process
	//TODO: log for helm local cache update fail
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = c.repoOptions.UpdateRepoLocalCache()
	}()

	index, err := c.loadIndex()
	if err != nil {
		return nil, err
	}
	var result = map[string]utils.CommonChartVersions{}
	for key, versions := range index {
		for _, item := range versions {
			commonCharts, ok := result[key]
			if ok {
				commonCharts = append(commonCharts, utils.CommonChartVersion{Name: key, Version: item.Version, URLType: "http", URL: item.URLs[0], RepoName: c.RepoName})
				result[key] = commonCharts
			} else {
				result[key] = utils.CommonChartVersions{{Name: key, Version: item.Version, URLType: "http", URL: item.URLs[0], Digest: item.Digest, RepoName: c.RepoName}}
			}
		}
	}
	wg.Wait()
	return result, nil
}
