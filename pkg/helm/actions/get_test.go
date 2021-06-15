package actions

import (
	"fmt"
	"testing"
)

func Test_GetHelmFullOverrideName(t *testing.T) {

	getOptions := &GetOptions{
		ReleaseName:       "helmops-test-operation",
		Namespace:         "helmops-system",
		KubernetesOptions: &KubernetesClient{ConfigFilePath: "/Users/lishijun1/.kube/config"},
	}
	name, err := getOptions.GetHelmFullOverrideName()
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(name)

}
