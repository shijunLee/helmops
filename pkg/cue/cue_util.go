package cue

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	helmopsapi "github.com/shijunLee/helmops/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/rest"
)

//ReleaseDef helm release def
type ReleaseDef struct {
	// release install name
	name string

	// release install namespace
	namespace string

	// is use auto upgrade
	autoUpgrade bool

	// release install kubernetes client
	client *rest.RESTClient
	mutex  sync.RWMutex

	// chart repo name for current chart
	chartRepo string

	// release install chart name
	chartName string

	// release install chart version
	chartVersion string

	// build values for release
	valueCUE string

	// build install options
	install *InstallOptions

	// build release update options
	update *UpgradeOptions

	// build release uninstall options
	uninstall *UninstallOptions
}

// InstallOptions install options
type InstallOptions struct {
	// DryRun controls whether the operation is prepared, but not executed.
	// If `true`, the upgrade is prepared but not performed.
	DryRun bool
	// Description install custom description
	Description string
	// SkipCRDs is skip crd when install
	SkipCRDs bool
	// TimeOut time out time
	Timeout time.Duration
	// NoHook do not use hook
	NoHook bool

	// install generatename
	GenerateName bool

	//CreateNamespace create namespace when install
	CreateNamespace bool

	// disable openapi validation on kubernetes install
	DisableOpenAPIValidation bool

	// is auth update
	IsUpgrade   bool
	WaitForJobs bool
	Replace     bool
	Wait        bool
}

// UpgradeOptions release upgrade options
type UpgradeOptions struct {

	// a purely informative flag that indicates whether this upgrade was done in "install" mode.
	//
	// Applications may use this to determine whether this Upgrade operation was done as part of a
	// pure upgrade (Upgrade.Install == false) or as part of an install-or-upgrade operation
	// (Upgrade.Install == true).
	//
	// Setting this to `true` will NOT cause `Upgrade` to perform an install if the release does not exist.
	// That process must be handled by creating an Install action directly. See cmd/upgrade.go for an
	// example of how this flag is used.
	Install bool
	// indicates that the operation is done in devel mode.
	Devel bool
	// is the namespace in which this operation should be performed.
	Namespace string
	// skips installing CRDs when install flag is enabled during upgrade
	SkipCRDs bool
	// is the timeout for this operation
	Timeout time.Duration
	// determines whether the wait operation should be performed after the upgrade is requested.
	Wait bool
	// disables hook processing if set to true.
	DisableHooks bool
	// controls whether the operation is prepared, but not executed.
	// If `true`, the upgrade is prepared but not performed.
	DryRun bool
	// Force will, if set to `true`, ignore certain warnings and perform the upgrade anyway.
	//
	// This should be used with caution.
	Force bool
	// will reset the values to the chart's built-ins rather than merging with existing.
	ResetValues bool
	// will re-use the user's last supplied values.
	ReuseValues bool
	// will (if true) recreate pods after a rollback.
	Recreate bool
	//  limits the maximum number of revisions saved per release
	MaxHistory int
	//  if true, will roll back on failure.
	Atomic bool
	//  will, if true, cause the upgrade to delete newly-created resources on a failed update.
	CleanupOnFail bool
	//  determines whether sub-notes are rendered in the chart.
	SubNotes bool
	//  is the description of this operation
	Description string
	//  controls whether OpenAPI validation is enforced.
	DisableOpenAPIValidation bool
	// wait for jobs end

	WaitForJobs bool

	// is upgrade crds
	UpgradeCRDs bool
}

//UninstallOptions release uninstall options
type UninstallOptions struct {
	DisableHooks       bool
	KeepHistory        bool
	Timeout            time.Duration
	Description        string
	DoNotDeleteRelease bool
}

// NewReleaseDef create new release def for cue template
func NewReleaseDef(name, namespace, chartName, chartVersion, chartRepoName string, autoUpgrade bool, client *rest.RESTClient, valueCUE string) *ReleaseDef {
	return &ReleaseDef{
		name:         name,
		namespace:    namespace,
		client:       client,
		mutex:        sync.RWMutex{},
		valueCUE:     valueCUE,
		autoUpgrade:  autoUpgrade,
		chartName:    chartName,
		chartVersion: chartVersion,
	}
}

// Parameter defines a parameter for cli from capability template
// TODO: need test this code
type Parameter struct {
	Name     string      `json:"name"`
	Short    string      `json:"short,omitempty"`
	Required bool        `json:"required,omitempty"`
	Default  interface{} `json:"default,omitempty"`
	Usage    string      `json:"usage,omitempty"`
	Type     cue.Kind    `json:"type,omitempty"`
	Alias    string      `json:"alias,omitempty"`
	JSONType string      `json:"jsonType,omitempty"`
}

//buildValues build helm chart values
// TODO: process out side values
func (r *ReleaseDef) buildValues(outSideValues map[string]interface{}) (map[string]interface{}, error) {
	ctx := cuecontext.New()
	values := ctx.CompileString(r.valueCUE)

	s, err := values.Struct()
	if err != nil {
		return nil, err
	}
	var paraDef cue.FieldInfo
	var found bool
	for i := 0; i < s.Len(); i++ {
		paraDef = s.Field(i)
		if paraDef.Name == "parameter" {
			found = true
			break
		}
	}
	if !found {
		return nil, errors.New("value is not found")
	}
	arguments, err := paraDef.Value.Struct()
	if err != nil {
		return nil, fmt.Errorf("arguments not defined as struct %w", err)
	}
	// parse each fields in the parameter fields
	var params []Parameter
	for i := 0; i < arguments.Len(); i++ {
		fi := arguments.Field(i)
		if fi.IsDefinition {
			continue
		}
		var param = Parameter{
			Name:     fi.Name,
			Required: !fi.IsOptional,
		}
		val := fi.Value
		param.Type = fi.Value.IncompleteKind()
		if def, ok := val.Default(); ok && def.IsConcrete() {
			param.Required = false
			param.Type = def.Kind()
			param.Default = GetDefault(def)
		}
		if param.Default == nil {
			param.Default = getDefaultByKind(param.Type)
		}

		params = append(params, param)
	}
	values = values.FillPath(cue.ParsePath("parameter"), outSideValues)
	resultValue := values.LookupPath(cue.ParsePath("output"))
	var result = map[string]interface{}{}
	err = resultValue.Decode(&result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// BuildReleaseWorkload
func (r *ReleaseDef) BuildReleaseWorkload(preValue map[string]interface{}) (*helmopsapi.HelmOperation, error) {
	var values = helmopsapi.CreateParam{
		Unstructured: unstructured.Unstructured{
			Object: map[string]interface{}{},
		},
	}
	createValue, err := r.buildValues(preValue)
	if err != nil {
		return nil, err
	}
	values.Unstructured.Object = createValue
	var install = r.install
	var upgrade = r.update
	var uninstall = r.uninstall
	var helmOperation = &helmopsapi.HelmOperation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.name,
			Namespace: r.namespace,
		},
		Spec: helmopsapi.HelmOperationSpec{
			Values:        values,
			AutoUpdate:    r.autoUpgrade,
			ChartRepoName: r.chartRepo,
			ChartName:     r.chartName,
			ChartVersion:  r.chartVersion,
			Create: helmopsapi.Create{
				Description:              install.Description,
				SkipCRDs:                 install.SkipCRDs,
				Timeout:                  install.Timeout,
				NoHook:                   install.NoHook,
				GenerateName:             install.GenerateName,
				CreateNamespace:          install.CreateNamespace,
				DisableOpenAPIValidation: install.DisableOpenAPIValidation,
				IsUpgrade:                install.IsUpgrade,
				WaitForJobs:              install.IsUpgrade,
				Replace:                  install.Replace,
				Wait:                     install.Wait,
			},
			Upgrade: helmopsapi.Upgrade{
				Install:                  upgrade.Install,
				Devel:                    upgrade.Devel,
				SkipCRDs:                 upgrade.SkipCRDs,
				Timeout:                  upgrade.Timeout,
				Wait:                     upgrade.Wait,
				DisableHooks:             upgrade.DisableHooks,
				Force:                    upgrade.Force,
				ResetValues:              upgrade.ResetValues,
				ReuseValues:              upgrade.ReuseValues,
				Recreate:                 upgrade.Recreate,
				MaxHistory:               upgrade.MaxHistory,
				Atomic:                   upgrade.Atomic,
				CleanupOnFail:            upgrade.CleanupOnFail,
				SubNotes:                 upgrade.SubNotes,
				Description:              upgrade.Description,
				DisableOpenAPIValidation: upgrade.DisableOpenAPIValidation,
				WaitForJobs:              upgrade.WaitForJobs,
				UpgradeCRDs:              upgrade.UpgradeCRDs,
			},
			Uninstall: helmopsapi.Uninstall{
				DisableHooks:       uninstall.DisableHooks,
				KeepHistory:        uninstall.KeepHistory,
				Timeout:            uninstall.Timeout,
				Description:        uninstall.Description,
				DoNotDeleteRelease: uninstall.DoNotDeleteRelease,
			},
		},
	}
	return helmOperation, nil
}

func getDefaultByKind(k cue.Kind) interface{} {
	// nolint:exhaustive
	switch k {
	case cue.IntKind:
		var d int64
		return d
	case cue.StringKind:
		var d string
		return d
	case cue.BoolKind:
		var d bool
		return d
	case cue.NumberKind, cue.FloatKind:
		var d float64
		return d
	default:
		// assume other cue kind won't be valid parameter
	}
	return nil
}

// GetDefault evaluate default Go value from CUE
func GetDefault(val cue.Value) interface{} {
	// nolint:exhaustive
	switch val.Kind() {
	case cue.IntKind:
		if d, err := val.Int64(); err == nil {
			return d
		}
	case cue.StringKind:
		if d, err := val.String(); err == nil {
			return d
		}
	case cue.BoolKind:
		if d, err := val.Bool(); err == nil {
			return d
		}
	case cue.NumberKind, cue.FloatKind:
		if d, err := val.Float64(); err == nil {
			return d
		}
	default:
	}
	return getDefaultByKind(val.Kind())
}
