package utils

import (
	"bytes"
	"errors"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
)

func DownloadChart(chartUrl, username, password string) (*chart.Chart, error) {
	return DownloadChartWithTLS(chartUrl, username, password, "", "", "", true)
}

// DownloadChart download helm chart to local cache folder
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
