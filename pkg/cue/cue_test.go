package cue

import (
	"encoding/json"
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

const testString = `
	  parameter: {
		data: [string]: string 
		configMapName: *"test-nginx-config" | string
		test:{
			group: "test-111" | string
			version: "test-222" | string
			resource: "test-3333"| string
            workloadName: "test-4444" | string
        }
        container:{
        	name: *"test-ccc" |  string
            rs: *0 | int
            imagePullSecrets:[
				{ name: *"test" | string }
			]
        }
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
			imagePullSecrets: parameter.container.imagePullSecrets
		  }  
	  }`

func Test_CUE_param(t *testing.T){
	ctx := cuecontext.New()
	values := ctx.CompileString(testString)

	s, err := values.Struct()
	if err != nil {
		t.Fatal(err)
		return
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
		t.Fatal(err)
		return
	}
	arguments, err := paraDef.Value.Struct()
	if err != nil {
		t.Fatal(err)
		return
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
	data ,err := json.Marshal(params)
	if err!=nil{
		t.Fatal(err)
		return
	}
	fmt.Println(string(data))
}

func Test_CUE_configMap(t *testing.T) {

	ctx := cuecontext.New()
	values := ctx.CompileString(testString)

	s, err := values.Struct()
	if err != nil {
		fmt.Println(err)
		t.Fatal(err)
		return
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
		t.Fatal("parameter not found")
		return
	}

	values = values.FillPath(cue.ParsePath("parameter.data"), map[string]string{
		"test1": "test2",
	})

	values = values.FillPath(cue.ParsePath("parameter.configMapName"), "test-cue-configmap")
	//value := values.Lookup("msg")
	result := values.LookupPath(cue.ParsePath("outputs.configmap"))
	var resultMap = map[string]interface{}{}
	err = result.Decode(&resultMap)
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
