/*
Copyright 2021 lishjun01@hotmail.com.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"strings"
	"text/template"
	"time"

	"github.com/Masterminds/sprig/v3"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"github.com/thedevsaddam/gojsonq"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	apimachinerymetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	helmopsv1alpha1 "github.com/shijunLee/helmops/api/v1alpha1"
	"github.com/shijunLee/helmops/pkg/cue"
	"github.com/shijunLee/helmops/pkg/helm/utils"
)

const (
	helmApplicationFinalizer = "finalizer.helmapplication.helmops.shijunlee.net"
)

// HelmApplicationReconciler reconciles a HelmApplication object
type HelmApplicationReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=helmops.shijunlee.net,resources=helmapplications,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=helmops.shijunlee.net,resources=helmapplications/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=helmops.shijunlee.net,resources=helmapplications/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *HelmApplicationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := r.Log.WithValues("helmapplication", req.NamespacedName)

	helmApplication := &helmopsv1alpha1.HelmApplication{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: req.Name, Namespace: req.Namespace}, helmApplication)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			l.Info("application not found")
			return ctrl.Result{}, nil
		}
		l.Error(err, "find helm application resource from client error", "ResourceName", req.Name, "ResourceName", req.Namespace)
		return ctrl.Result{}, err
	}
	if !helmApplication.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(helmApplication, helmApplicationFinalizer) {
			controllerutil.AddFinalizer(helmApplication, helmApplicationFinalizer)
			err = r.Client.Update(ctx, helmApplication)
			if err != nil {
				l.Error(err, "update helm application error")
				return ctrl.Result{}, err
			}
		}
	} else {
		if err = r.removeFinalizer(ctx, helmApplication); err != nil {
			l.Error(err, "remove finalizer error")
			return ctrl.Result{}, err
		}
		controllerutil.RemoveFinalizer(helmApplication, helmRepoFinalizer)
		err = r.Client.Update(ctx, helmApplication)
		if err != nil {
			l.Error(err, "update helm application error")
			return ctrl.Result{}, err
		}
	}
	l.Info("start application create")
	// not define steps ,return not process
	if len(helmApplication.Spec.Steps) == 0 {
		return ctrl.Result{}, nil
	}
	var values = map[string]interface{}{}
	for _, step := range helmApplication.Spec.Steps {

		cmp, err := r.getApplicationStepComponent(ctx, step)
		if err != nil {
			// if component not found ,add status
			if k8serrors.IsNotFound(err) {
				l.Error(err, "step install component not found")
				return ctrl.Result{}, nil
			}
			l.Error(err, "get step install component err")
			return ctrl.Result{}, err
		}
		operator := cmp.Spec.Operator
		if operator != nil {
			isExist, err := r.checkOperatorIsExist(ctx, helmApplication.Namespace, cmp)
			if err != nil {
				l.Error(err, "check operator is exist error")
				return ctrl.Result{}, err
			}
			// if operator is install do not install this step and do install others
			if isExist {
				continue
			}
		}

		helmOperation, err := r.buildStepReleaseHelmOperation(ctx, helmApplication.Namespace, step, values)
		if err != nil {
			//TODO: update status for the application ,build operation failed
			l.Error(err, "build helm operation error")
			return ctrl.Result{}, nil
		}
		exist, opsObj, err := r.checkOperationIsExist(ctx, helmOperation)
		if err != nil {
			l.Error(err, "check operation is exist error")
			return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
		}
		if !exist {
			err = r.Client.Create(ctx, helmOperation)
			if err != nil {
				l.Error(err, "create helm operation return error")
				return ctrl.Result{}, err
			}
			l.Info("success create operation , return loop and wait 1 second")
			return ctrl.Result{RequeueAfter: time.Second}, nil
		} else {
			// if spec not equal ,update helmOperation and wait 1 second
			// TODO: is need get deepequal values string
			if !reflect.DeepEqual(helmOperation.Spec, opsObj.Spec) {
				l.Info("helm operation spec not same,need update")
				opsObj.Spec = helmOperation.Spec
				err = r.Client.Update(ctx, opsObj)
				if err != nil {
					l.Error(err, "create helm operation return error")
					return ctrl.Result{}, err
				}
				return ctrl.Result{RequeueAfter: time.Second}, nil
			}
		}
		// do check componse status
		l.Info("start watch step release ready")
		isReady, err := r.watchStepReleaseReady(ctx, helmOperation, cmp)
		if err != nil {
			l.Error(err, "get cmp is ready error")
			return ctrl.Result{RequeueAfter: time.Second}, err
		}
		if !isReady {
			l.Info("release not ready,return loop and wait 1 second")
			return ctrl.Result{RequeueAfter: time.Second}, nil
		}

		l.Info("start get release return data")
		data, err := r.getStepReleaseReturnData(ctx, helmOperation, cmp)
		if err != nil {
			l.Error(err, "get return data error")
			return ctrl.Result{}, err
		}
		// TODO: update step return data

		values[step.ComponentReleaseName] = data
		printReturnValues(l, values)
	}

	return ctrl.Result{}, nil
}

//printReturnValues print debug return data info
func printReturnValues(log logr.Logger, data map[string]interface{}) {
	if data == nil {
		return
	}
	dataBytes, err := json.Marshal(data)
	if err != nil {
		log.Error(err, "marshal data err")
	}
	log.Info("get return data", "DataInfo", string(dataBytes))
}

//checkOperationIsExist check operation is install or not
func (r *HelmApplicationReconciler) checkOperationIsExist(ctx context.Context, helmOperation *helmopsv1alpha1.HelmOperation) (bool, *helmopsv1alpha1.HelmOperation, error) {
	var obj = &helmopsv1alpha1.HelmOperation{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: helmOperation.Name, Namespace: helmOperation.Namespace}, obj)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return false, nil, nil
		}
		return false, nil, err
	}

	return true, obj, nil

}

func (r *HelmApplicationReconciler) removeFinalizer(ctx context.Context, helmApplication *helmopsv1alpha1.HelmApplication) error {
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *HelmApplicationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&helmopsv1alpha1.HelmApplication{}).
		Complete(r)
}

//getApplicationStepComponent get application install step use component
func (r *HelmApplicationReconciler) getApplicationStepComponent(ctx context.Context, stepDef helmopsv1alpha1.ComponentStep) (*helmopsv1alpha1.HelmComponent, error) {
	l := r.Log.WithValues("stepReleaseName", stepDef.ComponentReleaseName, "Component", stepDef.ComponentName)
	if stepDef.ComponentName == "" {
		return nil, errors.New("not define step component name")
	}
	var helmComponent = &helmopsv1alpha1.HelmComponent{}
	err := r.Client.Get(ctx, types.NamespacedName{Namespace: utils.GetCurrentNameSpace(), Name: stepDef.ComponentName}, helmComponent)
	if err != nil {
		l.Error(err, "get helm component error")
		return nil, err
	}
	return helmComponent, nil
}

//processOperatorHelmReleaseInstall process operator helm release , get operator is install in the cluster or namespace , if not install will install
//TODO: if namespace scope operator use clusterrole or clusterrolebinding ,update clusterrole and clusterrolebinding ,not recreate it.
func (r *HelmApplicationReconciler) processOperatorHelmReleaseInstall(ctx context.Context, operation *helmopsv1alpha1.HelmOperation,
	helmComponent *helmopsv1alpha1.HelmComponent) {
	//TODO: this method not use ,to use for anthod time
}

//checkOperatorIsExist get the operator is install in the cluster
func (r *HelmApplicationReconciler) checkOperatorIsExist(ctx context.Context, namespace string, helmComponent *helmopsv1alpha1.HelmComponent) (bool, error) {
	l := r.Log.WithValues("helmapplication", namespace, "Component", helmComponent.Name)
	operator := helmComponent.Spec.Operator
	if operator == nil {
		return true, nil
	}
	if operator.MetaName != "" {
		obj := &unstructured.Unstructured{Object: map[string]interface{}{}}
		obj.SetGroupVersionKind(schema.GroupVersionKind{Group: operator.APIGroup, Kind: operator.APIKind, Version: operator.Version})
		var getInfo = types.NamespacedName{Name: operator.MetaName}
		if operator.WatchType == "Namespace" {
			getInfo.Namespace = namespace
		}
		err := r.Client.Get(ctx, getInfo, obj)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		return true, err
	}
	if operator.MatchLabels != nil {
		// if selector convert failed ,return false , reinstall the operator
		selecter, err := apimachinerymetav1.LabelSelectorAsSelector(operator.MatchLabels)
		if err != nil {
			l.Error(err, "convert meta v1 match labels selector to apimachinery selector error")
			return false, err
		}
		listOpts := &client.ListOptions{LabelSelector: selecter}
		if operator.WatchType == "Namespace" {
			listOpts.Namespace = namespace
		}
		var listKind = fmt.Sprintf("%sList", operator.APIKind)
		objList := &unstructured.UnstructuredList{Object: map[string]interface{}{}}
		objList.SetGroupVersionKind(schema.GroupVersionKind{Group: operator.APIGroup, Kind: listKind, Version: operator.Version})
		err = r.Client.List(ctx, objList, listOpts)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		if len(objList.Items) == 0 {
			return false, nil
		}
		return true, nil
	}

	return false, nil
}

// buildStepReleaseHelmOperation build install step releaseHelmOperation for application,the values is the before step return values
func (r *HelmApplicationReconciler) buildStepReleaseHelmOperation(ctx context.Context, namespace string,
	stepDef helmopsv1alpha1.ComponentStep, // helmComponent *helmopsv1alpha1.HelmComponent,
	values map[string]interface{}) (*helmopsv1alpha1.HelmOperation, error) {
	helmComponent, err := r.getApplicationStepComponent(ctx, stepDef)
	if err != nil {
		return nil, err
	}
	var refValues = map[string]interface{}{}
	for _, item := range stepDef.ValuesRefComponentRelease {
		value, ok := values[item]
		if ok {
			valueMap, ok := value.(map[string]interface{})
			if ok {
				for k, v := range valueMap {
					refValues[k] = v
				}
			}

		}
	}
	var componentInstall = helmComponent.Spec.Create
	var componentUpgrade = helmComponent.Spec.Upgrade
	var componentUninstall = helmComponent.Spec.Uninstall
	var installOptions = &cue.InstallOptions{
		DryRun:                   false,
		Description:              componentInstall.Description,
		SkipCRDs:                 componentInstall.SkipCRDs,
		Timeout:                  componentInstall.Timeout,
		NoHook:                   componentInstall.NoHook,
		GenerateName:             componentInstall.GenerateName,
		CreateNamespace:          componentInstall.CreateNamespace,
		DisableOpenAPIValidation: componentInstall.DisableOpenAPIValidation,
		IsUpgrade:                componentInstall.IsUpgrade,
		WaitForJobs:              componentInstall.WaitForJobs,
		Replace:                  componentInstall.Replace,
		Wait:                     componentInstall.Wait,
	}
	updgradeOptions := &cue.UpgradeOptions{
		Install:                  componentUpgrade.Install,
		Devel:                    componentUpgrade.Devel,
		Namespace:                namespace,
		SkipCRDs:                 componentUpgrade.SkipCRDs,
		Timeout:                  componentUpgrade.Timeout,
		Wait:                     componentUpgrade.Wait,
		DisableHooks:             componentUpgrade.DisableHooks,
		DryRun:                   false,
		Force:                    componentUpgrade.Force,
		ResetValues:              componentUpgrade.ResetValues,
		ReuseValues:              componentUpgrade.ReuseValues,
		Recreate:                 componentUpgrade.Recreate,
		MaxHistory:               componentUpgrade.MaxHistory,
		Atomic:                   componentUpgrade.Atomic,
		CleanupOnFail:            componentUpgrade.CleanupOnFail,
		SubNotes:                 componentUpgrade.SubNotes,
		Description:              componentUpgrade.Description,
		DisableOpenAPIValidation: componentUpgrade.DisableOpenAPIValidation,
		WaitForJobs:              componentUpgrade.WaitForJobs,
		UpgradeCRDs:              componentUpgrade.UpgradeCRDs,
	}
	uninstallOptions := &cue.UninstallOptions{
		DisableHooks:       componentUninstall.DisableHooks,
		KeepHistory:        componentUninstall.KeepHistory,
		Timeout:            componentUninstall.Timeout,
		Description:        componentUninstall.Description,
		DoNotDeleteRelease: componentUninstall.DoNotDeleteRelease,
	}

	var cueRef = cue.NewReleaseDef(stepDef.ComponentReleaseName, namespace,
		helmComponent.Spec.ChartName, helmComponent.Spec.ChartVersion, helmComponent.Spec.ChartRepoName, false, nil,
		helmComponent.Spec.ValuesTemplate.CUE.Template, installOptions, updgradeOptions, uninstallOptions)

	return cueRef.BuildReleaseWorkload(refValues)
}

// watchStepReleaseReady get the step release is ready
func (r *HelmApplicationReconciler) watchStepReleaseReady(ctx context.Context, operation *helmopsv1alpha1.HelmOperation,
	helmComponent *helmopsv1alpha1.HelmComponent) (bool, error) {

	// if not set stable status check ,return true,do nothing
	if helmComponent.Spec.StableStatus == nil {
		return true, nil
	}
	s := helmComponent.Spec.StableStatus
	//check operation is install
	err := r.Client.Get(ctx, types.NamespacedName{Name: operation.Name, Namespace: operation.Namespace}, operation)
	if err != nil {
		r.Log.Info("check helm component is error", "errInfo", err)
		return false, nil
	}
	releaseDefaultValueMap := getReleaseStatusMap(operation)
	resourceName, err := r.renderStringUseGoTemplate(ctx, s.Name, releaseDefaultValueMap)
	if err != nil {
		r.Log.Info("render string use go template in watch step release ready error", "errInfo", err)
		return false, err
	}

	if resourceName == "" {
		return false, errors.New("get resource name error")
	}
	r.Log.Info("Get status resource info", "ReleaseResourceName", resourceName, "Namespace", operation.Namespace,
		"Group", s.APIGroup, "Kind", s.Kind, "Version", s.Version)
	obj := &unstructured.Unstructured{Object: map[string]interface{}{}}
	obj.SetGroupVersionKind(schema.GroupVersionKind{Group: s.APIGroup, Version: s.Version, Kind: s.Kind})
	err = r.Client.Get(ctx, types.NamespacedName{Name: resourceName, Namespace: operation.Namespace}, obj)
	if err != nil {
		r.Log.Error(err, "get status object for release error", "ReleaseResourceName", resourceName, "Namespace", operation.Namespace,
			"Group", s.APIGroup, "Kind", s.Kind, "Version", s.Version)
		return false, nil
	}
	// user jsonq get the resource json path values and cmp the component yaml set values for the resource
	jsonData, err := json.Marshal(obj)
	if err != nil {
		r.Log.Info("obj marshal json error", "errInfo", err)
		return false, nil
	}

	data := gojsonq.New().FromString(string(jsonData)).Find(s.JSONPath)
	r.Log.Info("jsonq get value from path info", "JsonPath", s.JSONPath, "DataInfo", data)
	if data != nil {
		if s.Value != nil {
			if dataString, ok := data.(string); ok && dataString == *s.Value {
				return true, nil
			}
		} else if s.ValueJsonPath != nil {
			valueData := gojsonq.New().FromString(string(jsonData)).Find(*s.ValueJsonPath)
			r.Log.Info("jsonq get value from path info", "ValueJsonPath", s.JSONPath, "ValueDataInfo", valueData)
			if valueData != nil {
				if data == valueData {
					return true, nil
				}
			}
		}
	}

	return false, nil
}

//renderStringUseGoTemplate render template string use go template
func (r *HelmApplicationReconciler) renderStringUseGoTemplate(ctx context.Context, templateString string, values map[string]interface{}) (string, error) {
	var result = ""
	r.Log.Info("renderStringUseGoTemplate render info", "Template String", templateString, "Value", values)
	// use go txt template for get the resource name from config
	tt := template.New("gotpl").Funcs(sprig.TxtFuncMap())
	tt, err := tt.Parse(templateString)
	if err == nil {
		objBuff := &bytes.Buffer{}
		err = tt.Execute(objBuff, values)
		if err == nil {
			result = objBuff.String()
		} else {
			r.Log.Error(err, "template string with go template error")
			return "", err
		}
	} else {
		r.Log.Error(err, "template parse string with go template error")
		return "", err
	}
	return result, nil
}

//getStepReleaseReturnData get step release return data
func (r *HelmApplicationReconciler) getStepReleaseReturnData(ctx context.Context, operation *helmopsv1alpha1.HelmOperation,
	helmComponent *helmopsv1alpha1.HelmComponent) (map[string]interface{}, error) {
	status, err := r.watchStepReleaseReady(ctx, operation, helmComponent)
	if !status || err != nil {
		if err != nil {
			r.Log.Info("get helm component return data error, status not ready and get some error", "errInfo", err)
			return nil, err
		}
		r.Log.Info("get helm component status not ready in get stepReleaseReturnData")
		return nil, errors.New("get helm component status not ready in get stepReleaseReturnData")
	}
	var resultMap = map[string]interface{}{}
	returnValuesDefines := helmComponent.Spec.ReturnValues
	releaseDefineValues := getReleaseStatusMap(operation)
	for _, item := range returnValuesDefines {
		obj := &unstructured.Unstructured{Object: map[string]interface{}{}}
		obj.SetGroupVersionKind(schema.GroupVersionKind{Group: item.APIGroup, Version: item.Version, Kind: item.Kind})
		resourceName, err := r.renderStringUseGoTemplate(ctx, item.ResourceName, releaseDefineValues)
		if err != nil {
			r.Log.Info("get resource with go template in getStepReleaseReturnData error", "errorInfo", err)
			return nil, err
		}
		err = r.Client.Get(ctx, types.NamespacedName{Name: resourceName, Namespace: operation.Namespace}, obj)
		if err != nil {
			r.Log.Info("get return value from object error")
			return nil, err
		}
		// user jsonq get the resource json path values and cmp the component yaml set values for the resource
		jsonData, err := json.Marshal(obj)
		if err != nil {
			r.Log.Info("obj marshal json error", "errInfo", err)
			return nil, err
		}
		var values []interface{}
		for _, jsonPath := range item.JSONPaths {
			data := gojsonq.New().FromString(string(jsonData)).Find(jsonPath)
			if data != nil {
				values = append(values, data)
			}
		}
		var returnValue = ""
		if item.ValueTemplate != "" {
			r.Log.Info("get return data info with fmt", "ValueTemplate", item.ValueTemplate, "Values", values)
			returnValue = fmt.Sprintf(item.ValueTemplate, values...)
			resultMap[item.Name] = returnValue
		} else if len(values) == 1 {
			resultMap[item.Name] = values[0]
		} else {
			var stringStr []string
			for _, objectItem := range values {
				stringStr = append(stringStr, fmt.Sprintf("%v", objectItem))
			}
			if item.JoinSplit == "" {
				item.JoinSplit = "-"
			}
			resultMap[item.Name] = strings.Join(stringStr, item.JoinSplit)
		}

	}

	return resultMap, nil
}

// getReleaseStatusMap get release status map for operation
func getReleaseStatusMap(operation *helmopsv1alpha1.HelmOperation) map[string]interface{} {
	if operation == nil {
		return map[string]interface{}{}
	}
	releaseName := operation.Name
	releaseNamespace := operation.Namespace

	// set release param for template the resource name
	var releaseDefaultValueMap = map[string]interface{}{
		"Release": map[string]string{
			"Name":      releaseName,
			"Namespace": releaseNamespace,
		},
		"Chart": map[string]interface{}{
			"Name":    operation.Spec.ChartName,
			"Version": operation.Spec.ChartVersion,
		},
	}
	return releaseDefaultValueMap
}
