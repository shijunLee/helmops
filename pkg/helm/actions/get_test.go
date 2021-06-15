package actions

import (
	"fmt"
	"testing"
)

func Test_GetHelmFullOverrideName(t *testing.T) {

	getOptions := &GetOptions{
		ReleaseName:       "helmops-test-operation",
		Namespace:         "helmops-system",
		KubernetesOptions: &KubernetesClient{ConfigFilePath: "~/.kube/config"},
	}
	name, err := getOptions.GetHelmFullOverrideName()
	if err != nil {
		return
	}
	fmt.Println(name)

}
