package v1alpha1

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"sigs.k8s.io/yaml"

	"github.com/shijunLee/helmops/pkg/helm/utils"
)

var podlog = logf.Log.WithName("pod-resource")

// +kubebuilder:webhook:path=/mutate-v1-pod-pullsecretinject,mutating=true,sideEffects=None,failurePolicy=ignore,groups="",resources=pods,verbs=create;update,versions=v1,name=mpodpullsecretinject.kb.io,admissionReviewVersions={v1,v1beta1}

// +k8s:deepcopy-gen=false
// PodPullSecretInject process pod images pull secret inject
type PodPullSecretInject struct {
	Client                    client.Client
	DockerSecretConfigMapName string
	decoder                   *admission.Decoder
}

// podAnnotator adds an annotation to every incoming pods.
func (a *PodPullSecretInject) Handle(ctx context.Context, req admission.Request) admission.Response {
	pod := &corev1.Pod{}

	err := a.decoder.Decode(req, pod)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	var labels = pod.Labels
	_, ok := labels["helmops.shijunlee.net/exclude-inject"]
	if ok {
		return admission.Allowed("pod use labels for not inject secrets")
	}
	podlog.Info("default web hook for pod", "name", pod.Name, "namespace", pod.Namespace)
	if a.DockerSecretConfigMapName == "" {
		return admission.Allowed("not config inject secret for current process")
	}
	secretConfig := &corev1.ConfigMap{}
	err = a.Client.Get(context.TODO(), types.NamespacedName{
		Name:      a.DockerSecretConfigMapName,
		Namespace: utils.GetCurrentNameSpace(),
	}, secretConfig)
	if err != nil {
		return admission.Allowed("get secret config error,not need to process pod secrets")
	}
	configData := secretConfig.Data["dockerSecret"]
	secretConfigs := []SecretConfig{}
	err = yaml.Unmarshal([]byte(configData), &secretConfigs)
	if err != nil {
		podlog.Error(err, "get secret config error")
		return admission.Allowed("not need to process")
	}
	var secrets = map[string]string{}
	var dockerSecrets = []corev1.Secret{}
	for _, item := range secretConfigs {
		secretItem := &corev1.Secret{}
		err := a.Client.Get(context.TODO(), types.NamespacedName{
			Name:      item.SecretName,
			Namespace: item.SecretNamespace,
		}, secretItem)
		if err != nil {
			continue
		}

		if secretItem.Type == corev1.SecretTypeDockerConfigJson {
			secretData, ok := secretItem.Data[corev1.DockerConfigJsonKey]
			if ok {
				dockerSecrets = append(dockerSecrets, *secretItem)
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

	if len(secrets) == 0 {
		return admission.Allowed("not need to process")
	}
	var appendSecretName []string
	//TODO: set image here
	var containers = pod.Spec.Containers
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
		for _, imagePullSecret := range pod.Spec.ImagePullSecrets {
			if imagePullSecret.Name == item {
				contain = true
				break
			}
		}
		if !contain {
			pod.Spec.ImagePullSecrets = append(pod.Spec.ImagePullSecrets, corev1.LocalObjectReference{Name: item})

			for _, secret := range dockerSecrets {
				if secret.Name == item {
					if secret.Namespace != pod.Namespace {
						var cloneSecret = secret
						cloneSecret.ObjectMeta = metav1.ObjectMeta{}
						cloneSecret.Namespace = pod.Namespace
						if cloneSecret.Namespace == "" {
							cloneSecret.Namespace = "default"
						}
						cloneSecret.Name = pod.Name
						err = a.Client.Create(context.TODO(), &cloneSecret)
						if err != nil {
							podlog.Error(err, "clone create secret config error")
							continue
						}
					}
				}
			}
		}
	}
	marshaledPod, err := json.Marshal(pod)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledPod)
}

// InjectDecoder injects the decoder.
func (a *PodPullSecretInject) InjectDecoder(d *admission.Decoder) error {
	a.decoder = d
	return nil
}

// +k8s:deepcopy-gen=false
// DockerSecret the docker secret info
type DockerSecret struct {
	Auths map[string]DockerAuthInfo
}

// +k8s:deepcopy-gen=false
// DockerAuthInfo the docker auth info
type DockerAuthInfo struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Auth     string `json:"auth"`
}

// +k8s:deepcopy-gen=false
//SecretConfig the secret config info in configmap
type SecretConfig struct {
	SecretName      string `json:"secretName"`
	SecretNamespace string `json:"secretNamespace"`
}
