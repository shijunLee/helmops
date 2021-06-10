package actions

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gofrs/flock"
	"github.com/pkg/errors"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/repo"
	"sigs.k8s.io/yaml"

	"github.com/shijunLee/helmops/pkg/helm/utils"
	"github.com/shijunLee/helmops/pkg/log"
)

// RepoOptions helm repo info
type RepoOptions struct {
	RepoName string
	Username string
	Password string
	RepoURL  string
	RepoType string
	CertFile string
	KeyFile  string
	CAFile   string
	// InsecureSkipTLSVerify skip tls certificate checks for the chart download
	InsecureSkipTLSVerify bool
}

// ChartOpts helm chart options
type ChartOpts struct {
	// RepoOptions helm chart repo options
	RepoOptions *RepoOptions

	// ChartName install chart name
	ChartName string
	// ChartVersion install chart version
	ChartVersion string
	// ChartURL install chart url
	ChartURL string

	// InsecureSkipTLSVerify skip tls certificate checks for the chart download
	InsecureSkipTLSVerify bool

	//LocalPath chart local path
	LocalPath string
	//AuthInfo chartURL auth info
	AuthInfo AuthInfo

	ChartArchive *bytes.Buffer
	Chart        *chart.Chart
}

type AuthInfo struct {
	Username       string
	Password       string
	RootCAPath     string
	CertPath       string
	PrivateKeyPath string
}

func (c *ChartOpts) LoadChartFiles() ([]*loader.BufferedFile, error) {
	if c.Chart != nil {
		var files = c.Chart.Files
		var result []*loader.BufferedFile
		for _, item := range files {
			if item != nil {
				result = append(result, &loader.BufferedFile{
					Name: item.Name,
					Data: item.Data,
				})
			}
		}
		return result, nil
	}
	if c.LocalPath != "" {
		pathState, err := os.Stat(c.LocalPath)
		if err == nil {
			files := []*loader.BufferedFile{}
			if pathState.IsDir() {
				var localPath = c.LocalPath
				if !strings.HasSuffix(localPath, "/") {
					localPath = fmt.Sprintf("%s/", localPath)
				}
				err := filepath.Walk(c.LocalPath, func(path string, f os.FileInfo, err error) error {
					if f == nil {
						return err
					}
					if f.IsDir() {
						return nil
					}
					data, err := ioutil.ReadFile(path)
					if err == nil {
						name := strings.TrimPrefix(path, localPath)
						files = append(files, &loader.BufferedFile{Name: name, Data: data})
					}
					return nil
				})
				if err != nil {
					return nil, err
				}
				return files, nil
			} else {
				data, err := ioutil.ReadFile(c.LocalPath)
				if err != nil {
					return nil, err
				}
				return loader.LoadArchiveFiles(bytes.NewReader(data))
			}
		}
	}
	if c.ChartArchive != nil {
		return loader.LoadArchiveFiles(c.ChartArchive)
	}

	if c.ChartURL != "" {
		bytesBuffer, err := utils.DownloadChartArchive(c.ChartURL, c.AuthInfo.Username, c.AuthInfo.Password, c.AuthInfo.RootCAPath,
			c.AuthInfo.CertPath, c.AuthInfo.PrivateKeyPath, c.InsecureSkipTLSVerify)
		if err != nil {
			return nil, err
		}
		return loader.LoadArchiveFiles(bytesBuffer)
	}

	if c.RepoOptions != nil {
		url, err := FindChartInAuthAndTLSRepoURL(c.RepoOptions.RepoURL, c.RepoOptions.Username, c.RepoOptions.Password,
			c.ChartName, c.ChartVersion, c.RepoOptions.CertFile, c.RepoOptions.KeyFile, c.RepoOptions.CAFile, c.RepoOptions.InsecureSkipTLSVerify,
			getter.All(&cli.EnvSettings{}))
		if err != nil {
			return nil, err
		}
		bytesBuffer, err := utils.DownloadChartArchive(url, c.RepoOptions.Username, c.RepoOptions.Password,
			c.RepoOptions.CAFile, c.RepoOptions.CertFile, c.RepoOptions.KeyFile, false)
		if err != nil {
			return nil, err
		}
		return loader.LoadArchiveFiles(bytesBuffer)
	}
	return nil, errors.New("load chart error ,chart load method not config")
}

//LoadChart  get chart from config
func (c *ChartOpts) LoadChart() (*chart.Chart, error) {
	if c.Chart != nil {
		return c.Chart, nil
	}
	if c.LocalPath != "" {
		_, err := os.Stat(c.LocalPath)
		if err == nil {
			return loader.Load(c.LocalPath)
		}
	}
	if c.ChartArchive != nil {
		return loader.LoadArchive(c.ChartArchive)
	}
	if c.ChartURL != "" {
		return utils.DownloadChartWithTLS(c.ChartURL, c.AuthInfo.Username, c.AuthInfo.Password, c.AuthInfo.RootCAPath,
			c.AuthInfo.CertPath, c.AuthInfo.PrivateKeyPath, c.InsecureSkipTLSVerify)
	}
	if c.RepoOptions != nil {
		url, err := FindChartInAuthAndTLSRepoURL(c.RepoOptions.RepoURL, c.RepoOptions.Username, c.RepoOptions.Password,
			c.ChartName, c.ChartVersion, c.RepoOptions.CertFile, c.RepoOptions.KeyFile, c.RepoOptions.CAFile, c.RepoOptions.InsecureSkipTLSVerify,
			getter.All(&cli.EnvSettings{}))
		if err != nil {
			return nil, err
		}
		return utils.DownloadChartWithTLS(url, c.RepoOptions.Username, c.RepoOptions.Password,
			c.RepoOptions.CAFile, c.RepoOptions.CertFile, c.RepoOptions.KeyFile, false)
	}

	return nil, errors.New("load chart error ,chart load method not config")
}

var errNoRepositories = errors.New("no repositories found. You must add one before updating")

type repoAddOptions struct {
	name                 string
	url                  string
	username             string
	password             string
	forceUpdate          bool
	allowDeprecatedRepos bool

	// TODO: this cert file not impl
	certFile              string
	keyFile               string
	caFile                string
	insecureSkipTLSverify bool

	repoFile  string
	repoCache string

	// Deprecated, but cannot be removed until Helm 4
	deprecatedNoUpdate bool
}

type repoUpdateOptions struct {
	update    func([]*repo.ChartRepository) []error
	repoFile  string
	repoCache string
}

func UpdateChartRepoToLocal(repoName string) error {
	var settings = cli.New()
	o := &repoUpdateOptions{update: updateCharts}
	f, err := repo.LoadFile(o.repoFile)
	switch {
	case os.IsNotExist(errors.Cause(err)):
		return errNoRepositories
	case err != nil:
		return errors.Wrapf(err, "failed loading file: %s", o.repoFile)
	case len(f.Repositories) == 0:
		return errNoRepositories
	}

	var repos []*repo.ChartRepository
	for _, cfg := range f.Repositories {
		if repoName != "" && cfg.Name != repoName {
			continue
		}
		r, err := repo.NewChartRepository(cfg, getter.All(settings))
		if err != nil {
			return err
		}
		if o.repoCache != "" {
			r.CachePath = o.repoCache
		}
		repos = append(repos, r)
		// while update only one repo stop the loop wile add current repo info
		if repoName != "" && cfg.Name == repoName {
			break
		}
	}

	errs := o.update(repos)

	if len(errs) > 0 {
		var err error = errors.New("update chart repo return error,")
		for _, item := range errs {
			err = errors.Wrap(err, fmt.Sprintf("%s,\n", item.Error()))
		}
		return err
	}
	return nil
}

func updateCharts(repos []*repo.ChartRepository) []error {
	var errorResponses []error
	var wg sync.WaitGroup
	for _, re := range repos {
		wg.Add(1)
		go func(re *repo.ChartRepository) {
			defer wg.Done()
			if _, err := re.DownloadIndexFile(); err != nil {
				err = errors.Wrap(err, fmt.Sprintf("repo %s download index file error", re.Config.Name))
				errorResponses = append(errorResponses, err)
			}
		}(re)
	}
	wg.Wait()
	return errorResponses
}
func (r *RepoOptions) UpdateRepoLocalCache() error {
	return UpdateChartRepoToLocal(r.RepoName)
}

func (r *RepoOptions) ChartRepoToLocal(isForceUpdate bool) error {
	var settings = cli.New()
	o := &repoAddOptions{}
	o.name = r.RepoName
	o.url = r.RepoURL
	o.repoFile = settings.RepositoryConfig
	o.repoCache = settings.RepositoryCache
	o.username = r.Username
	o.password = r.Password
	o.caFile = r.CAFile
	o.certFile = r.CertFile
	o.keyFile = r.KeyFile
	o.forceUpdate = isForceUpdate
	log.GlobalLog.WithName("chartmuseum-chartrepotolocal").Info("repo config setting address", "RepositoryConfigDir", o.repoFile)
	log.GlobalLog.WithName("chartmuseum-chartrepotolocal").Info("repo cache setting address", "RepositoryConfigDir", o.repoCache)
	// Ensure the file directory exists as it is required for file locking
	err := os.MkdirAll(filepath.Dir(o.repoFile), os.ModePerm)
	if err != nil && !os.IsExist(err) {
		log.GlobalLog.WithName("chartmuseum-chartrepotolocal").Error(err, "make dir error")
		return err
	}

	// Acquire a file lock for process synchronization
	fileLock := flock.New(strings.Replace(o.repoFile, filepath.Ext(o.repoFile), ".lock", 1))
	lockCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	locked, err := fileLock.TryLockContext(lockCtx, time.Second)
	if err == nil && locked {
		defer fileLock.Unlock()
	}
	if err != nil {
		log.GlobalLog.WithName("chartmuseum-chartrepotolocal").Error(err, "create cache file error")
		return err
	}

	b, err := ioutil.ReadFile(o.repoFile)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	var f repo.File
	if err := yaml.Unmarshal(b, &f); err != nil {
		return err
	}

	c := repo.Entry{
		Name:                  o.name,
		URL:                   o.url,
		Username:              o.username,
		Password:              o.password,
		CertFile:              o.certFile,
		KeyFile:               o.keyFile,
		CAFile:                o.caFile,
		InsecureSkipTLSverify: o.insecureSkipTLSverify,
	}

	// If the repo exists do one of two things:
	// 1. If the configuration for the name is the same continue without error
	// 2. When the config is different require --force-update
	if !o.forceUpdate && f.Has(o.name) {
		return nil
	}

	cr, err := repo.NewChartRepository(&c, getter.All(settings))
	if err != nil {
		return err
	}

	if o.repoCache != "" {
		cr.CachePath = o.repoCache
	}
	if _, err := cr.DownloadIndexFile(); err != nil {
		return errors.Wrapf(err, "looks like %q is not a valid chart repository or cannot be reached", o.url)
	}

	f.Update(&c)

	if err := f.WriteFile(o.repoFile, 0644); err != nil {
		return err
	}
	return nil
}

func (r *RepoOptions) AddChartRepoToLocal() error {
	return r.ChartRepoToLocal(false)
}

func (r *RepoOptions) GetLatestRepoIndex() (*repo.IndexFile, error) {
	repoIndexURL := fmt.Sprintf("%s/index.yaml", r.RepoURL)
	var indexFile = &IndexFile{
		IndexFile:  repo.NewIndexFile(),
		ServerInfo: &ServerInfo{},
	}

	err := utils.HttpGetStruct(repoIndexURL, map[string]string{}, indexFile,
		utils.WithBasicAuth(r.Username, r.Password),
		utils.WithInsecureSkipVerifyTLS(r.InsecureSkipTLSVerify),
		utils.WithTLSClientConfig(r.KeyFile, r.CAFile, r.CertFile))
	if err != nil {
		return nil, err
	}
	return indexFile.IndexFile, nil
}

// ServerInfo helm repo server info
type ServerInfo struct {
	ContextPath string `json:"contextPath,omitempty"`
}

type IndexFile struct {
	*repo.IndexFile
	ServerInfo *ServerInfo `json:"serverInfo"`
}

// FindChartInAuthAndTLSRepoURL finds chart in chart repository pointed by repoURL
// without adding repo to repositories, like FindChartInRepoURL,
// but it also receives credentials and TLS verify flag for the chart repository.
// TODO Helm 4, FindChartInAuthAndTLSRepoURL should be integrated into FindChartInAuthRepoURL.
func FindChartInAuthAndTLSRepoURL(repoURL, username, password, chartName, chartVersion, certFile, keyFile, caFile string, insecureSkipTLSverify bool, getters getter.Providers) (string, error) {

	// Download and write the index file to a temporary location
	buf := make([]byte, 20)
	rand.Read(buf)
	name := strings.ReplaceAll(base64.StdEncoding.EncodeToString(buf), "/", "-")

	c := repo.Entry{
		URL:                   repoURL,
		Username:              username,
		Password:              password,
		CertFile:              certFile,
		KeyFile:               keyFile,
		CAFile:                caFile,
		Name:                  name,
		InsecureSkipTLSverify: insecureSkipTLSverify,
	}
	r, err := repo.NewChartRepository(&c, getters)
	if err != nil {
		return "", err
	}
	idx, err := r.DownloadIndexFile()
	if err != nil {
		return "", errors.Wrapf(err, "looks like %q is not a valid chart repository or cannot be reached", repoURL)
	}

	// Read the index file for the repository to get chart information and return chart URL
	repoIndex, err := repo.LoadIndexFile(idx)
	if err != nil {
		return "", err
	}

	errMsg := fmt.Sprintf("chart %q", chartName)
	if chartVersion != "" {
		errMsg = fmt.Sprintf("%s version %q", errMsg, chartVersion)
	}
	cv, err := repoIndex.Get(chartName, chartVersion)
	if err != nil {
		return "", errors.Errorf("%s not found in %s repository", errMsg, repoURL)
	}

	if len(cv.URLs) == 0 {
		return "", errors.Errorf("%s has no downloadable URLs", errMsg)
	}

	chartURL := cv.URLs[0]

	absoluteChartURL, err := repo.ResolveReferenceURL(repoURL, chartURL)
	if err != nil {
		return "", errors.Wrap(err, "failed to make chart URL absolute")
	}

	return absoluteChartURL, nil
}
