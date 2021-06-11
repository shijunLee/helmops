package v1alpha1

import (
	"context"
	"encoding/json"
	"strings"

	corev1 "k8s.io/api/core/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/yaml"

	"github.com/shijunLee/helmops/pkg/helm/utils"
)

var excludeInject = "helmops.shijunlee.net/exclude-inject"

type PodWebHook struct {
	corev1.Pod
}

type SecretConfig struct {
	SecretName      string `json:"secretName"`
	SecretNamespace string `json:"secretNamespace"`
}

var dockerSecretConfigMapName = ""
var mgrClient client.Client

// log is for logging in this package.
var podlog = logf.Log.WithName("pod-resource")

func (r *PodWebHook) SetupWebhookWithManager(mgr ctrl.Manager, dockerConfigMapName string) error {
	dockerSecretConfigMapName = dockerConfigMapName
	mgrClient = mgr.GetClient()
	return ctrl.NewWebhookManagedBy(mgr).
		For(&corev1.Pod{}).
		Complete()
}

var _ webhook.Defaulter = &PodWebHook{}

//+kubebuilder:webhook:path=/mutate-helmops-shijunlee-net-v1alpha1-podwebhook,mutating=true,failurePolicy=fail,sideEffects=None,groups="",resources=pods,verbs=create;update,versions=v1,name=mpods.kb.io,admissionReviewVersions={v1,v1beta1}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *PodWebHook) Default() {
	var labels = r.Pod.Labels
	_, ok := labels["helmops.shijunlee.net/exclude-inject"]
	if ok {
		return
	}
	podlog.Info("default web hook for pod", "name", r.Name, "namespace", r.Namespace)
	if dockerSecretConfigMapName == "" {
		return
	}
	secretConfig := &corev1.ConfigMap{}
	err := mgrClient.Get(context.TODO(), types.NamespacedName{
		Name:      dockerSecretConfigMapName,
		Namespace: utils.GetCurrentNameSpace(),
	}, secretConfig)
	if err != nil {
		return
	}
	configData := secretConfig.Data["dockerSecret"]
	secretConfigs := []SecretConfig{}
	err = yaml.Unmarshal([]byte(configData), &secretConfigs)
	if err != nil {
		podlog.Error(err, "get secret config error")
		return
	}
	var secrets = map[string]string{}
	for _, item := range secretConfigs {
		secretItem := &corev1.Secret{}
		err := mgrClient.Get(context.TODO(), types.NamespacedName{
			Name:      item.SecretName,
			Namespace: item.SecretNamespace,
		}, secretItem)
		if err != nil {
			continue
		}
		if secretItem != nil {
			if secretItem.Type == corev1.SecretTypeDockerConfigJson {
				secretData, ok := secretItem.Data[corev1.DockerConfigJsonKey]
				if ok {
					var dockerSecret = &DockerSecret{}
					err := json.Unmarshal(secretData, dockerSecret)
					if err == nil {
						for k := range dockerSecret.Auths {
							secrets[k] = secretItem.Name
						}
					}
				}
			}
		}
	}
	if len(secrets) == 0 {
		return
	}
	var appendSecretName []string
	//TODO: set image here
	var containers = r.Pod.Spec.Containers
	for _, item := range containers {
		var image = item.Image
		for k, v := range secrets {
			if strings.HasPrefix(image, k) {
				appendSecretName = append(appendSecretName, v)
			}
		}
	}
	for _, item := range appendSecretName {
		var contain = false
		for _, imagePullSecret := range r.Pod.Spec.ImagePullSecrets {
			if imagePullSecret.Name == item {
				contain = true
				break
			}
		}
		if !contain {
			r.Pod.Spec.ImagePullSecrets = append(r.Pod.Spec.ImagePullSecrets, corev1.LocalObjectReference{Name: item})
		}
	}
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-helmops-shijunlee-net-v1alpha1-helmrepo,mutating=false,failurePolicy=fail,sideEffects=None,groups=helmops.shijunlee.net,resources=helmrepos,verbs=create;update,versions=v1alpha1,name=vpod.kb.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Validator = &PodWebHook{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *PodWebHook) ValidateCreate() error {
	podlog.Info("validate create", "name", r.Name)

	return r.commonValidate()
}

func (r *PodWebHook) commonValidate() error {
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *PodWebHook) ValidateUpdate(old runtime.Object) error {
	podlog.Info("validate update", "name", r.Name)

	return r.commonValidate()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *PodWebHook) ValidateDelete() error {
	podlog.Info("validate delete", "name", r.Name)
	return nil
}

type DockerSecret struct {
	Auths map[string]DockerAuthInfo
}

type DockerAuthInfo struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Auth     string `json:"auth"`
}
