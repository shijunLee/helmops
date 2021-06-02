package cue

import (
	"fmt"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"gopkg.in/yaml.v3"
)

func Test_CUE(t *testing.T) {
	const config = `
msg:   "Hello \(place)!"
place: string | *"world" // "world" is the default.
`
	ctx := cuecontext.New()
	values := ctx.CompileString(config)

	values = values.FillPath(cue.ParsePath("place"), "you")
	//value := values.Lookup("msg")
	result := values.LookupPath(cue.ParsePath("msg"))
	str, err := result.String()
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(str)
}

func Test_CUE_configMap(t *testing.T) {
	const testString = `
	  parameter: {
		data: [string]: string 
		configMapName: *"test-nginx-config" | string
	  } 
	  // trait template can have multiple outputs in one trait
	  outputs: configmap: {
		  apiVersion: "v1"
		  kind:       "ConfigMap"
		  metadata:
			name: parameter.configMapName
		  data: {
			for k, v in parameter.data {
			  "\(k)": v
			}, 
		  }  
	  }`
	ctx := cuecontext.New()
	values := ctx.CompileString(testString)

	values = values.FillPath(cue.ParsePath("parameter.data"), map[string]string{
		"test1": "test2",
	})
	values = values.FillPath(cue.ParsePath("parameter.configMapName"), "test-cue-configmap")
	//value := values.Lookup("msg")
	result := values.LookupPath(cue.ParsePath("outputs.configmap"))
	var resultMap = map[string]interface{}{}
	err := result.Decode(&resultMap)
	if err != nil {
		fmt.Println(err)
		t.Fatal(err)
		return
	}
	data, err := yaml.Marshal(resultMap)
	if err != nil {
		fmt.Println(err)
		t.Fatal(err)
		return
	}
	fmt.Println(string(data))
}
