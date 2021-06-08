package utils

import (
	"io/ioutil"
	"os"
)

const (
	currentNamespacePath string = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
)

// GetCurrentNameSpace get current namespace in pod ,if debug please set ENV HELM_OPS_NAMESPACE for debug
func GetCurrentNameSpace() string {
	currentNameSpace := os.Getenv("HELM_OPS_NAMESPACE")
	if currentNameSpace != "" {
		return currentNameSpace
	} else {
		_, err := os.Stat(currentNamespacePath)
		if err != nil {
			return currentNameSpace
		} else {
			data, err := ioutil.ReadFile(currentNamespacePath)
			if err != nil {
				return currentNameSpace
			} else {
				return string(data)
			}
		}
	}
}
