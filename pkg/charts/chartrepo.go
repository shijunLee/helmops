package charts

type ChartRepo interface {
	GetChartLastVersion(chartName string) (string, error)
	GetChartVersionUrl(chartName, chartVersion string) (url, pathType string, err error)
	CheckChartExist(chartName, version string) bool
}
