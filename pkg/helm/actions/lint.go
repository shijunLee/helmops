package actions

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Masterminds/semver"
	"github.com/asaskevich/govalidator"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
	helmactions "helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/engine"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/lint/support"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
)

const (
	chartFileName  = "Chart.yaml"
	valuesFileName = "values.yaml"
	schemaFilename = "values.schema.json"
	templatesPath  = "templates/"
)

var (
	crdHookSearch     = regexp.MustCompile(`"?helm\.sh/hook"?:\s+crd-install`)
	releaseTimeSearch = regexp.MustCompile(`\.Release\.Time`)
)

type LintOptions struct {
	Strict        bool
	WithSubCharts bool
}

func (i *LintOptions) Run(namespace string, chartOpts *ChartOpts) (*helmactions.LintResult, error) {
	if chartOpts == nil {
		return nil, errors.New("charts not config,can not run lint")
	}

	valueOpts := &values.Options{}
	var settings = &cli.EnvSettings{}

	getters := getter.All(settings)
	vals, err := valueOpts.MergeValues(getters)
	if err != nil {
		return nil, err
	}
	if chartOpts.LocalPath != "" {
		var lintActions = helmactions.NewLint()
		lintActions.Namespace = namespace
		lintActions.Strict = i.Strict
		lintActions.WithSubcharts = i.WithSubCharts
		result := lintActions.Run([]string{chartOpts.LocalPath}, vals)
		return result, nil
	}
	files, err := chartOpts.LoadChartFiles()
	if err != nil {
		return nil, errors.Wrapf(err, "load archive file error")
	}

	linter := &support.Linter{}
	err = linterChartFile(linter, files)
	if err != nil {
		return nil, errors.Wrapf(err, "linter chart file error")
	}
	valuesWithOverrides(linter, files, vals)
	templates(linter, files, vals, namespace, i.Strict)
	dependencies(linter, files)
	lowestTolerance := support.ErrorSev
	if i.Strict {
		lowestTolerance = support.WarningSev
	}
	result := &helmactions.LintResult{}
	result.Messages = append(result.Messages, linter.Messages...)
	result.TotalChartsLinted++
	for _, msg := range linter.Messages {
		if msg.Severity >= lowestTolerance {
			result.Errors = append(result.Errors, msg.Err)
		}
	}
	return result, nil
}

func linterChartFile(linter *support.Linter, f []*loader.BufferedFile) error {
	chartInfo, err := loadChartMetadata(f)
	validChartFile := linter.RunLinterRule(support.ErrorSev, chartFileName, validateChartYamlFormat(err))

	// Guard clause. Following linter rules require a parsable ChartFile
	if !validChartFile {
		return errors.Wrapf(err, "load archive file error")
	}

	// type check for Chart.yaml . ignoring error as any parse
	// errors would already be caught in the above load function
	chartFileForTypeCheck, _ := loadChartFileForTypeCheck(f)

	linter.RunLinterRule(support.ErrorSev, chartFileName, validateChartName(chartInfo))
	// Chart metadata
	linter.RunLinterRule(support.ErrorSev, chartFileName, validateChartAPIVersion(chartInfo))
	linter.RunLinterRule(support.ErrorSev, chartFileName, validateChartVersionType(chartFileForTypeCheck))
	linter.RunLinterRule(support.ErrorSev, chartFileName, validateChartVersion(chartInfo))
	linter.RunLinterRule(support.ErrorSev, chartFileName, validateChartAppVersionType(chartFileForTypeCheck))
	linter.RunLinterRule(support.ErrorSev, chartFileName, validateChartMaintainer(chartInfo))
	linter.RunLinterRule(support.ErrorSev, chartFileName, validateChartSources(chartInfo))
	linter.RunLinterRule(support.InfoSev, chartFileName, validateChartIconPresence(chartInfo))
	linter.RunLinterRule(support.ErrorSev, chartFileName, validateChartIconURL(chartInfo))
	linter.RunLinterRule(support.ErrorSev, chartFileName, validateChartType(chartInfo))
	linter.RunLinterRule(support.ErrorSev, chartFileName, validateChartDependencies(chartInfo))
	return nil
}

func valuesWithOverrides(linter *support.Linter, f []*loader.BufferedFile, values map[string]interface{}) {
	fileExists := linter.RunLinterRule(support.InfoSev, valuesFileName, loadValuesFiles(f))
	if !fileExists {
		return
	}
	linter.RunLinterRule(support.ErrorSev, valuesFileName, validateValuesFile(f, values))
}

func validateValuesFile(f []*loader.BufferedFile, overrides map[string]interface{}) error {
	var item = loadFileFromBufferedFile(valuesFileName, f)
	var values = map[string]interface{}{}
	err := yaml.Unmarshal(item.Data, values)
	if err != nil {
		return errors.Wrap(err, "unable to parse YAML")
	}

	// Helm 3.0.0 carried over the values linting from Helm 2.x, which only tests the top
	// level values against the top-level expectations. Subchart values are not linted.
	// We could change that. For now, though, we retain that strategy, and thus can
	// coalesce tables (like reuse-values does) instead of doing the full chart
	// CoalesceValues.
	var schemeFile = loadFileFromBufferedFile(schemaFilename, f)
	if schemeFile == nil {
		return nil
	}
	return chartutil.ValidateAgainstSingleSchema(values, schemeFile.Data)
}

func validateChartVersionType(data map[string]interface{}) error {
	return isStringValue(data, "version")
}

func validateChartAppVersionType(data map[string]interface{}) error {
	return isStringValue(data, "appVersion")
}

func loadValuesFiles(f []*loader.BufferedFile) error {
	var item = loadFileFromBufferedFile(valuesFileName, f)
	if item == nil {
		return errors.New("values.yaml not found")
	}
	return nil
}

func loadChartMetadata(f []*loader.BufferedFile) (*chart.Metadata, error) {
	var item = loadFileFromBufferedFile(chartFileName, f)
	if item == nil {
		return nil, errors.New("chart.yaml not found")
	}

	jsonData, err := k8syaml.ToJSON(item.Data)
	if err != nil {
		return nil, err
	}
	var metadata = &chart.Metadata{}
	err = json.Unmarshal(jsonData, &metadata)
	if err != nil {
		return nil, err
	}
	return metadata, nil
}

func loadFileFromBufferedFile(fileName string, f []*loader.BufferedFile) *loader.BufferedFile {
	for _, item := range f {
		if item.Name == fileName {
			return item
		}
	}
	return nil
}

// loadChartFileForTypeCheck loads the Chart.yaml
// in a generic form of a map[string]interface{}, so that the type
// of the values can be checked
func loadChartFileForTypeCheck(f []*loader.BufferedFile) (map[string]interface{}, error) {
	var data []byte
	for _, item := range f {
		if item.Name == chartFileName {
			data = item.Data
		}
	}
	if len(data) == 0 {
		return nil, errors.New("Chart.yaml not found")
	}
	y := make(map[string]interface{})
	err := yaml.Unmarshal(data, &y)
	return y, err
}

func isStringValue(data map[string]interface{}, key string) error {
	value, ok := data[key]
	if !ok {
		return nil
	}
	valueType := fmt.Sprintf("%T", value)
	if valueType != "string" {
		return errors.Errorf("%s should be of type string but it's of type %s", key, valueType)
	}
	return nil
}

func validateChartYamlFormat(chartFileError error) error {
	if chartFileError != nil {
		return errors.Errorf("unable to parse YAML\n\t%s", chartFileError.Error())
	}
	return nil
}

func validateChartName(cf *chart.Metadata) error {
	if cf.Name == "" {
		return errors.New("name is required")
	}
	return nil
}

func validateChartAPIVersion(cf *chart.Metadata) error {
	if cf.APIVersion == "" {
		return errors.New("apiVersion is required. The value must be either \"v1\" or \"v2\"")
	}

	if cf.APIVersion != chart.APIVersionV1 && cf.APIVersion != chart.APIVersionV2 {
		return fmt.Errorf("apiVersion '%s' is not valid. The value must be either \"v1\" or \"v2\"", cf.APIVersion)
	}

	return nil
}

func validateChartVersion(cf *chart.Metadata) error {
	if cf.Version == "" {
		return errors.New("version is required")
	}

	version, err := semver.NewVersion(cf.Version)

	if err != nil {
		return errors.Errorf("version '%s' is not a valid SemVer", cf.Version)
	}

	c, err := semver.NewConstraint(">0.0.0-0")
	if err != nil {
		return err
	}
	valid, msg := c.Validate(version)

	if !valid && len(msg) > 0 {
		return errors.Errorf("version %v", msg[0])
	}

	return nil
}

func validateChartMaintainer(cf *chart.Metadata) error {
	for _, maintainer := range cf.Maintainers {
		if maintainer.Name == "" {
			return errors.New("each maintainer requires a name")
		} else if maintainer.Email != "" && !govalidator.IsEmail(maintainer.Email) {
			return errors.Errorf("invalid email '%s' for maintainer '%s'", maintainer.Email, maintainer.Name)
		} else if maintainer.URL != "" && !govalidator.IsURL(maintainer.URL) {
			return errors.Errorf("invalid url '%s' for maintainer '%s'", maintainer.URL, maintainer.Name)
		}
	}
	return nil
}

func validateChartSources(cf *chart.Metadata) error {
	for _, source := range cf.Sources {
		if source == "" || !govalidator.IsRequestURL(source) {
			return errors.Errorf("invalid source URL '%s'", source)
		}
	}
	return nil
}

func validateChartIconPresence(cf *chart.Metadata) error {
	if cf.Icon == "" {
		return errors.New("icon is recommended")
	}
	return nil
}

func validateChartIconURL(cf *chart.Metadata) error {
	if cf.Icon != "" && !govalidator.IsRequestURL(cf.Icon) {
		return errors.Errorf("invalid icon URL '%s'", cf.Icon)
	}
	return nil
}

func validateChartDependencies(cf *chart.Metadata) error {
	if len(cf.Dependencies) > 0 && cf.APIVersion != chart.APIVersionV2 {
		return fmt.Errorf("dependencies are not valid in the Chart file with apiVersion '%s'. They are valid in apiVersion '%s'", cf.APIVersion, chart.APIVersionV2)
	}
	return nil
}

func validateChartType(cf *chart.Metadata) error {
	if len(cf.Type) > 0 && cf.APIVersion != chart.APIVersionV2 {
		return fmt.Errorf("chart type is not valid in apiVersion '%s'. It is valid in apiVersion '%s'", cf.APIVersion, chart.APIVersionV2)
	}
	return nil
}

// Templates lints the templates in the Linter.
func templates(linter *support.Linter, f []*loader.BufferedFile, values map[string]interface{}, namespace string, strict bool) {

	templatesDirExist := linter.RunLinterRule(support.WarningSev, templatesPath, validateTemplatesDir(f))

	// Templates directory is optional for now
	if !templatesDirExist {
		return
	}

	// Load chart and parse templates
	chart, err := loader.LoadFiles(f)

	chartLoaded := linter.RunLinterRule(support.ErrorSev, templatesPath, err)

	if !chartLoaded {
		return
	}

	options := chartutil.ReleaseOptions{
		Name:      "test-release",
		Namespace: namespace,
	}

	cvals, err := chartutil.CoalesceValues(chart, values)
	if err != nil {
		return
	}
	valuesToRender, err := chartutil.ToRenderValues(chart, cvals, options, nil)
	if err != nil {
		linter.RunLinterRule(support.ErrorSev, templatesPath, err)
		return
	}
	var e engine.Engine
	e.LintMode = true
	renderedContentMap, err := e.Render(chart, valuesToRender)

	renderOk := linter.RunLinterRule(support.ErrorSev, templatesPath, err)

	if !renderOk {
		return
	}

	/* Iterate over all the templates to check:
	- It is a .yaml file
	- All the values in the template file is defined
	- {{}} include | quote
	- Generated content is a valid Yaml file
	- Metadata.Namespace is not set
	*/
	var fpath = templatesPath
	for _, template := range chart.Templates {
		fileName, data := template.Name, template.Data
		fpath = fileName

		linter.RunLinterRule(support.ErrorSev, fpath, validateAllowedExtension(fileName))
		// These are v3 specific checks to make sure and warn people if their
		// chart is not compatible with v3
		linter.RunLinterRule(support.WarningSev, fpath, validateNoCRDHooks(data))
		linter.RunLinterRule(support.ErrorSev, fpath, validateNoReleaseTime(data))

		// We only apply the following lint rules to yaml files
		if filepath.Ext(fileName) != ".yaml" || filepath.Ext(fileName) == ".yml" {
			continue
		}

		// NOTE: disabled for now, Refs https://github.com/helm/helm/issues/1463
		// Check that all the templates have a matching value
		//linter.RunLinterRule(support.WarningSev, fpath, validateNoMissingValues(templatesPath, valuesToRender, preExecutedTemplate))

		// NOTE: disabled for now, Refs https://github.com/helm/helm/issues/1037
		// linter.RunLinterRule(support.WarningSev, fpath, validateQuotes(string(preExecutedTemplate)))

		renderedContent := renderedContentMap[path.Join(chart.Name(), fileName)]
		if strings.TrimSpace(renderedContent) != "" {
			linter.RunLinterRule(support.WarningSev, fpath, validateTopIndentLevel(renderedContent))

			decoder := k8syaml.NewYAMLOrJSONDecoder(strings.NewReader(renderedContent), 4096)

			// Lint all resources if the file contains multiple documents separated by ---
			for {
				// Even though K8sYamlStruct only defines a few fields, an error in any other
				// key will be raised as well
				var yamlStruct *K8sYamlStruct

				err := decoder.Decode(&yamlStruct)
				if err == io.EOF {
					break
				}

				// If YAML linting fails, we sill progress. So we don't capture the returned state
				// on this linter run.
				linter.RunLinterRule(support.ErrorSev, fpath, validateYamlContent(err))

				if yamlStruct != nil {
					// NOTE: set to warnings to allow users to support out-of-date kubernetes
					// Refs https://github.com/helm/helm/issues/8596
					linter.RunLinterRule(support.WarningSev, fpath, validateMetadataName(yamlStruct))
					//	linter.RunLinterRule(support.WarningSev, fpath, validateNoDeprecations(yamlStruct))

					linter.RunLinterRule(support.ErrorSev, fpath, validateMatchSelector(yamlStruct, renderedContent))
				}
			}
		}
	}
}

// Dependencies runs lints against a chart's dependencies
//
// See https://github.com/helm/helm/issues/7910
func dependencies(linter *support.Linter, f []*loader.BufferedFile) {
	c, err := loader.LoadFiles(f)
	if !linter.RunLinterRule(support.ErrorSev, "", validateChartFormat(err)) {
		return
	}

	linter.RunLinterRule(support.ErrorSev, linter.ChartDir, validateDependencyInMetadata(c))
	linter.RunLinterRule(support.WarningSev, linter.ChartDir, validateDependencyInChartsDir(c))
}

// validateTopIndentLevel checks that the content does not start with an indent level > 0.
//
// This error can occur when a template accidentally inserts space. It can cause
// unpredictable errors dependening on whether the text is normalized before being passed
// into the YAML parser. So we trap it here.
//
// See https://github.com/helm/helm/issues/8467
func validateTopIndentLevel(content string) error {
	// Read lines until we get to a non-empty one
	scanner := bufio.NewScanner(bytes.NewBufferString(content))
	for scanner.Scan() {
		line := scanner.Text()
		// If line is empty, skip
		if strings.TrimSpace(line) == "" {
			continue
		}
		// If it starts with one or more spaces, this is an error
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			return fmt.Errorf("document starts with an illegal indent: %q, which may cause parsing problems", line)
		}
		// Any other condition passes.
		return nil
	}
	return scanner.Err()
}

// Validation functions
func validateTemplatesDir(f []*loader.BufferedFile) error {
	var hasDirectory = false
	for _, item := range f {
		if strings.HasPrefix(item.Name, templatesPath) {
			hasDirectory = true
		}
	}
	if hasDirectory {
		return nil
	}
	return errors.New("not a directory")
}

func validateAllowedExtension(fileName string) error {
	ext := filepath.Ext(fileName)
	validExtensions := []string{".yaml", ".yml", ".tpl", ".txt"}

	for _, b := range validExtensions {
		if b == ext {
			return nil
		}
	}

	return errors.Errorf("file extension '%s' not valid. Valid extensions are .yaml, .yml, .tpl, or .txt", ext)
}

func validateYamlContent(err error) error {
	return errors.Wrap(err, "unable to parse YAML")
}

func validateMetadataName(obj *K8sYamlStruct) error {
	if len(obj.Metadata.Name) == 0 || len(obj.Metadata.Name) > 253 {
		return fmt.Errorf("object name must be between 0 and 253 characters: %q", obj.Metadata.Name)
	}
	// This will return an error if the characters do not abide by the standard OR if the
	// name is left empty.
	if err := chartutil.ValidateMetadataName(obj.Metadata.Name); err != nil {
		return errors.Wrapf(err, "object name does not conform to Kubernetes naming requirements: %q", obj.Metadata.Name)
	}
	return nil
}

func validateNoCRDHooks(manifest []byte) error {
	if crdHookSearch.Match(manifest) {
		return errors.New("manifest is a crd-install hook. This hook is no longer supported in v3 and all CRDs should also exist the crds/ directory at the top level of the chart")
	}
	return nil
}

func validateNoReleaseTime(manifest []byte) error {
	if releaseTimeSearch.Match(manifest) {
		return errors.New(".Release.Time has been removed in v3, please replace with the `now` function in your templates")
	}
	return nil
}

// validateMatchSelector ensures that template specs have a selector declared.
// See https://github.com/helm/helm/issues/1990
func validateMatchSelector(yamlStruct *K8sYamlStruct, manifest string) error {
	switch yamlStruct.Kind {
	case "Deployment", "ReplicaSet", "DaemonSet", "StatefulSet":
		// verify that matchLabels or matchExpressions is present
		if !(strings.Contains(manifest, "matchLabels") || strings.Contains(manifest, "matchExpressions")) {
			return fmt.Errorf("a %s must contain matchLabels or matchExpressions, and %q does not", yamlStruct.Kind, yamlStruct.Metadata.Name)
		}
	}
	return nil
}

func validateDependencyInChartsDir(c *chart.Chart) (err error) {
	dependencies := map[string]struct{}{}
	missing := []string{}
	for _, dep := range c.Dependencies() {
		dependencies[dep.Metadata.Name] = struct{}{}
	}
	for _, dep := range c.Metadata.Dependencies {
		if _, ok := dependencies[dep.Name]; !ok {
			missing = append(missing, dep.Name)
		}
	}
	if len(missing) > 0 {
		err = fmt.Errorf("chart directory is missing these dependencies: %s", strings.Join(missing, ","))
	}
	return err
}

func validateDependencyInMetadata(c *chart.Chart) (err error) {
	dependencies := map[string]struct{}{}
	missing := []string{}
	for _, dep := range c.Metadata.Dependencies {
		dependencies[dep.Name] = struct{}{}
	}
	for _, dep := range c.Dependencies() {
		if _, ok := dependencies[dep.Metadata.Name]; !ok {
			missing = append(missing, dep.Metadata.Name)
		}
	}
	if len(missing) > 0 {
		err = fmt.Errorf("chart metadata is missing these dependencies: %s", strings.Join(missing, ","))
	}
	return err
}

func validateChartFormat(chartError error) error {
	if chartError != nil {
		return errors.Errorf("unable to load chart\n\t%s", chartError)
	}
	return nil
}

// K8sYamlStruct stubs a Kubernetes YAML file.
//
// DEPRECATED: In Helm 4, this will be made a private type, as it is for use only within
// the rules package.
type K8sYamlStruct struct {
	APIVersion string `json:"apiVersion"`
	Kind       string
	Metadata   k8sYamlMetadata
}

type k8sYamlMetadata struct {
	Namespace string
	Name      string
}
