package git

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/shijunLee/helmops/pkg/helm/utils"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/pkg/errors"

	"github.com/go-git/go-git/v5/plumbing/transport"

	"github.com/go-git/go-git/v5"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

var (
	GitPathExistErr    = errors.New("this local path has git dir,can not use this path")
	GetPathNotExistErr = errors.New("this local path is not a git repo")
)

type Repo struct {
	URL             string
	Username        string
	Password        string
	Token           string
	LocalPath       string
	Branch          string
	authMethod      transport.AuthMethod
	RepoName        string
	InsecureSkipTLS bool
}

func NewRepo(url, username, password, token, branch, localPath, repoName string, insecureSkipTLS bool) (*Repo, error) {
	g := &Repo{
		URL:             url,
		Username:        username,
		Password:        password,
		Token:           token,
		LocalPath:       localPath,
		Branch:          branch,
		RepoName:        repoName,
		InsecureSkipTLS: insecureSkipTLS,
	}

	if g.Username != "" && g.Password != "" {
		if strings.HasPrefix(strings.ToLower(g.URL), "http") {
			g.authMethod = &githttp.BasicAuth{Username: g.Username, Password: g.Password}
		} else {
			g.authMethod = &ssh.Password{User: g.Username, Password: g.Password}
		}
	}
	if g.Token != "" {
		g.authMethod = &githttp.TokenAuth{Token: g.Token}
	}
	err := g.Clone()
	if err != nil {
		if err == GitPathExistErr {
			err = g.Pull()
			if err != nil {
				return nil, err
			}
		}
		return nil, err
	}
	return g, nil
}

func (g *Repo) checkPathCanClone() error {
	fileInfo, err := os.Stat(g.LocalPath)
	if err != nil {
		return err
	}
	if !fileInfo.IsDir() {
		return errors.New("local path not a dir")
	}
	var gitCachePath = path.Join(g.LocalPath, ".git")
	_, err = os.Stat(gitCachePath)
	if err == nil {
		return GitPathExistErr
	}
	return nil
}

func (g *Repo) checkPathCanPull() error {

	fileInfo, err := os.Stat(g.LocalPath)
	if err != nil {
		return err
	}
	if !fileInfo.IsDir() {
		return errors.New("local path not a dir")
	}
	var gitCachePath = path.Join(g.LocalPath, ".git")
	fileInfo, err = os.Stat(gitCachePath)
	if err != nil {
		return err
	}
	if !fileInfo.IsDir() {
		return GetPathNotExistErr
	}
	return nil
}

func (g *Repo) Clone() error {
	err := g.checkPathCanClone()
	if err != nil {
		return err
	}
	var cloneOptions = &git.CloneOptions{
		URL:             g.URL,
		Progress:        os.Stdout,
		InsecureSkipTLS: g.InsecureSkipTLS,
		ReferenceName:   plumbing.NewBranchReferenceName(g.Branch),
	}
	cloneOptions.Auth = g.authMethod
	_, err = git.PlainClone(g.LocalPath, false, cloneOptions)
	if err != nil {
		return err
	}
	return nil
}

func (g *Repo) Pull() error {
	err := g.checkPathCanPull()
	if err != nil {
		return err
	}
	fileInfo, err := os.Stat(g.LocalPath)
	if err != nil {
		return nil
	}
	if fileInfo.IsDir() {
		r, err := git.PlainOpen(g.LocalPath)
		if err != nil {
			return err
		}
		var fetchOptions = &git.FetchOptions{
			InsecureSkipTLS: g.InsecureSkipTLS,
		}
		fetchOptions.Auth = g.authMethod
		err = r.Fetch(fetchOptions)
		if err != nil && !(err == git.NoErrAlreadyUpToDate) {
			return err
		}
		return nil
	}
	return nil
}

//Diff can not use
func (g *Repo) Diff() error {
	err := g.checkPathCanPull()
	if err != nil {
		return err
	}
	//r, err := git.PlainOpen(g.LocalPath)
	//if err != nil {
	//	return err
	//}

	//git.GrepResult{}
	return nil
}

//GetChartLastVersion get git last version for chart
// the git file tree must like 'chart/{chartname}/{chartversion}'
func (g *Repo) GetChartLastVersion(chartName string) (string, error) {
	versions, err := g.getChartVersions(chartName)
	if err != nil {
		return "", err
	}
	return utils.GetLatestSemver(versions)
}

func (g *Repo) CheckChartExist(chartName, version string) bool {
	versions, err := g.getChartVersions(chartName)
	if err != nil {
		return false
	}
	for _, item := range versions {
		if item == version {
			return true
		}
	}
	return false
}

func (g *Repo) GetChartVersionUrl(chartName, chartVersion string) (url, pathType string, err error) {
	var chartPath = path.Join(g.LocalPath, g.RepoName, "charts", chartName, chartVersion)
	_, err = os.Stat(chartPath)
	if err != nil {
		return "", "", err
	}
	return chartPath, "file", nil
}

func (g *Repo) getChartVersions(chartName string) ([]string, error) {
	err := g.Pull()
	if err != nil {
		return nil, err
	}
	var chartPath = path.Join(g.LocalPath, g.RepoName, "charts", chartName)
	fileInfo, err := os.Stat(chartPath)
	if err != nil {
		return nil, err
	}
	if !fileInfo.IsDir() {
		return nil, errors.New("current chart file path not a dir")
	}
	var versions []string
	err = filepath.WalkDir(chartPath, func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			relativePaths := strings.TrimPrefix(path, chartPath)
			relativePaths = strings.TrimPrefix(relativePaths, "/")
			relativePathArray := strings.Split(relativePaths, "/")
			if len(relativePathArray) == 1 {
				versions = append(versions, relativePaths)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(versions) == 0 {
		return nil, errors.New("not found the chart from git repo")
	}
	return versions, nil
}
