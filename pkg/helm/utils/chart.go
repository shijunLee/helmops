package utils

import (
	"bytes"
	"errors"

	"github.com/Masterminds/semver/v3"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
)

//DownloadChart download helm chart to local cache folder
func DownloadChart(chartUrl, username, password string) (*chart.Chart, error) {
	return DownloadChartWithTLS(chartUrl, username, password, "", "", "", true)
}

//DownloadChartWithTLS download helm chart to local cache folder
// chartUrl chart download url
// username the chart repo auth username
// password the chart repo auth password
func DownloadChartWithTLS(chartUrl, username, password string,
	caPath, certPath, privateKeyPath string, insecureSkipTLSVerify bool) (*chart.Chart, error) {
	buff, err := DownloadChartArchive(chartUrl, username, password, caPath, certPath, privateKeyPath, insecureSkipTLSVerify)
	if err != nil {
		return nil, err
	}
	return loader.LoadArchive(buff)
}

//DownloadChartArchive download chart Archive and get chart Archive bytes buffer
func DownloadChartArchive(chartUrl, username, password string, caPath, certPath, privateKeyPath string, insecureSkipTLSVerify bool) (*bytes.Buffer, error) {
	var opts []HttpRequestOptions
	if username != "" && password != "" {
		opts = append(opts, WithBasicAuth(username, password))
	}
	if caPath != "" && certPath != "" && privateKeyPath != "" {
		opts = append(opts, WithTLSClientConfig(privateKeyPath, caPath, certPath))
	}
	if insecureSkipTLSVerify {
		opts = append(opts, WithInsecureSkipVerifyTLS(insecureSkipTLSVerify))
	}
	data, stateCode, _, err := HttpGet(chartUrl, nil, opts...)
	if err != nil {
		return nil, err
	}
	if stateCode > 400 {
		//todo if 401 return add token support
		return nil, errors.New("download chart return 401 from repo")
	}
	return bytes.NewBuffer(data), nil
}

//CommonChartVersion common chart info,notice this code will be rebuild
type CommonChartVersion struct {
	Name     string
	Version  string
	URL      string
	URLType  string
	Digest   string
	RepoName string
}

// CommonChartVersions is a list of versioned chart references.
// Implements a sorter on Version.
type CommonChartVersions []CommonChartVersion

// Len returns the length.
func (c CommonChartVersions) Len() int { return len(c) }

// Swap swaps the position of two items in the versions slice.
func (c CommonChartVersions) Swap(i, j int) { c[i], c[j] = c[j], c[i] }

// Less returns true if the version of entry a is less than the version of entry b.
func (c CommonChartVersions) Less(a, b int) bool {
	// Failed parse pushes to the back.
	i, err := semver.NewVersion(c[a].Version)
	if err != nil {
		return true
	}
	j, err := semver.NewVersion(c[b].Version)
	if err != nil {
		return false
	}
	return i.LessThan(j)
}
