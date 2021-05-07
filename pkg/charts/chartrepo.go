package charts

import (
	"sort"
	"time"

	"github.com/shijunLee/helmops/pkg/helm/utils"

	"github.com/pkg/errors"
	"github.com/shijunLee/helmops/pkg/charts/chartmuseum"
	git "github.com/shijunLee/helmops/pkg/charts/git"
)

var (
	RepoTypeNotSupportErr = errors.New("repo type not support")
)

const (
	repoTypeGit         = "Git"
	repoTypeChartMuseum = "ChartMuseum"
	defaultBranch       = "master"
)

type ChartRepoInterface interface {
	GetChartLastVersion(chartName string) (string, error)
	GetChartVersionUrl(chartName, chartVersion string) (url, pathType string, err error)
	CheckChartExist(chartName, version string) bool
	ListCharts() (map[string]utils.CommonChartVersions, error)
}

type ChartRepo struct {
	Name            string
	Type            string
	URL             string
	Username        string
	Password        string
	Token           string
	Branch          string
	InsecureSkipTLS bool
	Period          int
	// the default local cache , all repo will same
	LocalCache string
	// the cert not support current
	Cert       []byte
	RootCA     []byte
	PrivateKey []byte
	Operation  ChartRepoInterface
	CancelChan chan int
}

func NewChartRepo(name, repoType, url, username, password, token, branch, localCache string, insecureSkipTLS bool, period int) (*ChartRepo, error) {
	if repoType != "Git" && repoType != "ChartMuseum" {
		return nil, RepoTypeNotSupportErr
	}
	if branch == "" {
		branch = defaultBranch
	}
	var operation ChartRepoInterface
	var err error
	switch repoType {
	case repoTypeGit:
		operation, err = git.NewRepo(url, username, password, token, branch, localCache, name, insecureSkipTLS)
	case repoTypeChartMuseum:
		operation, err = chartmuseum.NewChartMuseum(url, username, password, name, insecureSkipTLS)
	}
	if err != nil {
		return nil, err
	}
	c := &ChartRepo{
		Name:            name,
		Type:            repoType,
		URL:             url,
		Username:        username,
		Password:        password,
		Token:           token,
		Branch:          branch,
		InsecureSkipTLS: insecureSkipTLS,
		Period:          period,
		LocalCache:      localCache,
		Operation:       operation,
		CancelChan:      make(chan int),
	}
	return c, nil
}

func (c *ChartRepo) Close() {
	c.CancelChan <- 1
}

func (c *ChartRepo) StartTimerJobs(callbackFunc func(chart *utils.CommonChartVersion, err error)) {
	timeTicker := time.NewTicker(time.Duration(c.Period) * time.Second)
	defer timeTicker.Stop()
	for {
		select {
		case <-timeTicker.C:
			chartVersions, err := c.Operation.ListCharts()
			if err != nil {
				callbackFunc(nil, err)
				continue
			}
			for _, versions := range chartVersions {
				sort.Sort(versions)
				var item = versions[0]
				callbackFunc(&item, nil)
			}
		case <-c.CancelChan:
			return
		}
	}

}
