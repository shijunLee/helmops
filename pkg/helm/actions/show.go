package actions

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	helmactions "helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	"sigs.k8s.io/yaml"
)

var (
	readmeFileNames = []string{"readme.md", "readme.txt", "readme"}
)

const (
	ShowIcon helmactions.ShowOutputFormat = "icon"
)

type ShowOptions struct {
	Devel        bool
	OutputFormat helmactions.ShowOutputFormat
	chart        *chart.Chart // for testing
	ChartOpts    *ChartOpts
}

func (s *ShowOptions) Run() (string, error) {
	if s.chart == nil {
		if s.ChartOpts == nil {
			return "", errors.New("show chart not config")
		}
		chartInfo, err := s.ChartOpts.LoadChart()
		if err != nil {
			return "", errors.Wrapf(err, "load chart error in run show command")
		}
		s.chart = chartInfo
	}
	cf, err := yaml.Marshal(s.chart.Metadata)
	if err != nil {
		return "", err
	}

	var out strings.Builder
	if s.OutputFormat == helmactions.ShowChart || s.OutputFormat == helmactions.ShowAll {
		fmt.Fprintf(&out, "%s\n", cf)
	}

	if (s.OutputFormat == helmactions.ShowValues || s.OutputFormat == helmactions.ShowAll) && s.chart.Values != nil {
		if s.OutputFormat == helmactions.ShowAll {
			fmt.Fprintln(&out, "---")
		}
		for _, f := range s.chart.Raw {
			if f.Name == chartutil.ValuesfileName {
				fmt.Fprintln(&out, string(f.Data))
			}
		}
	}

	if s.OutputFormat == helmactions.ShowReadme || s.OutputFormat == helmactions.ShowAll {
		if s.OutputFormat == helmactions.ShowAll {
			fmt.Fprintln(&out, "---")
		}
		readme := findReadme(s.chart.Files)
		if readme == nil {
			return out.String(), nil
		}
		fmt.Fprintf(&out, "%s\n", readme.Data)
	}

	if s.OutputFormat == ShowIcon {
		if s.chart.Metadata != nil {
			iconPath := s.chart.Metadata.Icon
			if iconPath != "" {
				if strings.HasPrefix(strings.ToLower(iconPath), "http://") || strings.HasPrefix(strings.ToLower(iconPath), "https://") {
					fmt.Fprintf(&out, "%s\n", iconPath)
				}
			}
		}
	}

	return out.String(), nil
}

func findReadme(files []*chart.File) (file *chart.File) {
	for _, file := range files {
		for _, n := range readmeFileNames {
			if strings.EqualFold(file.Name, n) {
				return file
			}
		}
	}
	return nil
}
