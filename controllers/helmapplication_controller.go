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
	"strings"
	"text/template"

	"k8s.io/apimachinery/pkg/util/json"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/Masterminds/sprig/v3"
	"github.com/pkg/errors"
	"github.com/shijunLee/helmops/pkg/cue"
	"github.com/thedevsaddam/gojsonq"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/go-logr/logr"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	apimachinerymetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	helmopsv1alpha1 "github.com/shijunLee/helmops/api/v1alpha1"
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
// TODO(user): Modify the Reconcile function to compare the state specified by
// the HelmApplication object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.7.2/pkg/reconcile
func (r *HelmApplicationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("helmapplication", req.NamespacedName)

	helmApplication := &helmopsv1alpha1.HelmApplication{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: req.Name, Namespace: req.Namespace}, helmApplication)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Error(err, "find helm application resource from client error", "ResourceName", req.Name, "ResourceName", req.Namespace)
		return ctrl.Result{}, err
	}
	if !helmApplication.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(helmApplication, helmApplicationFinalizer) {
			controllerutil.AddFinalizer(helmApplication, helmApplicationFinalizer)
			err = r.Client.Update(ctx, helmApplication)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		if err = r.removeFinalizer(ctx, helmApplication); err != nil {
			return ctrl.Result{}, err
		}
		controllerutil.RemoveFinalizer(helmApplication, helmRepoFinalizer)
	}

	// your logic here

	return ctrl.Result{}, nil
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

//processOperatorHelmReleaseInstall process operator helm release , get operator is install in the cluster or namespace , if not install will install
//TODO: if namespace scope operator use clusterrole or clusterrolebinding ,update clusterrole and clusterrolebinding ,not recreate it.
func (r *HelmApplicationReconciler) processOperatorHelmReleaseInstall(ctx context.Context, operation *helmopsv1alpha1.HelmOperation,
	helmComponent *helmopsv1alpha1.HelmComponent) {
	//TODO: this method not use ,to use for anthod time
}

//checkOperatorIsExist get the operator is install in the cluster
func (r *HelmApplicationReconciler) checkOperatorIsExist(ctx context.Context, namespace string, helmComponent *helmopsv1alpha1.HelmComponent) (bool, error) {
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
			r.Log.Error(err, "convert meta v1 match labels selector to apimachinery selector error")
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
	stepDef *helmopsv1alpha1.ComponentStep, helmComponent *helmopsv1alpha1.HelmComponent,
	values map[string]interface{}) (*helmopsv1alpha1.HelmOperation, error) {
	var cueRef = cue.NewReleaseDef(stepDef.ComponentReleaseName, namespace,
		helmComponent.Spec.ChartName, helmComponent.Spec.ChartVersion, helmComponent.Spec.ChartRepoName, false, nil,
		helmComponent.Spec.ValuesTemplate.CUE.Template)
	return cueRef.BuildReleaseWorkload(values)
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
	resourceName, err := renderStringUseGoTemplate(ctx, s.Name, releaseDefaultValueMap)
	if err != nil {
		r.Log.Info("render string use go template in watch step release ready error", "errInfo", err)
		return false, err
	}

	if resourceName == "" {
		return false, errors.New("get resource name error")
	}
	obj := &unstructured.Unstructured{Object: map[string]interface{}{}}
	obj.SetGroupVersionKind(schema.GroupVersionKind{Group: s.APIGroup, Version: s.Version, Kind: s.Kind})
	err = r.Client.Get(ctx, types.NamespacedName{Name: resourceName, Namespace: operation.Namespace}, obj)
	if err != nil {
		return false, nil
	}
	// user jsonq get the resource json path values and cmp the component yaml set values for the resource
	jsonData, err := json.Marshal(obj)
	if err != nil {
		r.Log.Info("obj marshal json error", "errInfo", err)
		return false, nil
	}

	data := gojsonq.New().FromString(string(jsonData)).Find(s.JSONPath)
	if data != nil {
		if dataString, ok := data.(string); ok && dataString == s.Value {
			return true, nil
		}
	}

	return false, nil
}

//renderStringUseGoTemplate render template string use go template
func renderStringUseGoTemplate(ctx context.Context, templateString string, values map[string]interface{}) (string, error) {
	var result = ""
	// use go txt template for get the resource name from config
	tt := template.New("gotpl").Funcs(sprig.TxtFuncMap())
	tt, err := tt.Parse(templateString)
	if err == nil {
		objBuff := &bytes.Buffer{}
		err = tt.Execute(objBuff, values)
		if err == nil {
			result = objBuff.String()
		} else {
			return "", err
		}
	} else {
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
		resourceName, err := renderStringUseGoTemplate(ctx, item.ResourceName, releaseDefineValues)
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
