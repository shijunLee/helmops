package v1alpha1

import (
	"bytes"
	"encoding/json"
	"io"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

//CreateParam tpaas middleware app create param
// +k8s:deepcopy-gen=false
type CreateParam struct {
	unstructured.Unstructured `json:",inline"`
}

//GetObjectKind imp runtime.Object
func (c *CreateParam) GetObjectKind() schema.ObjectKind {
	return c
}

//SetGroupVersionKind imp schema.ObjectKind
func (c *CreateParam) SetGroupVersionKind(kind schema.GroupVersionKind) {

}

// GroupVersionKind returns the stored group, version, and kind of an object, or nil if the object does
// not expose or provide these fields.
func (c *CreateParam) GroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{Group: "helmops.shijunlee.net", Kind: "CreateParam"}
}

//DeepCopyObject deep copy object from CreateParam imp runtime.Object
func (c *CreateParam) DeepCopyObject() runtime.Object {
	return c.DeepCopy()
}

//DeepCopy  deep copy object from CreateParam imp runtime.Object
func (c *CreateParam) DeepCopy() *CreateParam {
	if c == nil {
		return nil
	}
	out := new(CreateParam)
	*out = *c
	out.Object = runtime.DeepCopyJSON(c.Object)
	return out
}

//DeepCopyInto deep copy object from CreateParam imp runtime.Object
func (c *CreateParam) DeepCopyInto(out *CreateParam) {
	clone := c.DeepCopy()
	*out = *clone
	return
}

// MarshalJSON ensures that the unstructured object produces proper
// JSON when passed to Go's standard JSON library.
func (c *CreateParam) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	err := CreateParamJSONScheme.Encode(c, &buf)
	return buf.Bytes(), err
}

// UnmarshalJSON ensures that the unstructured object properly decodes
// JSON when passed to Go's standard JSON library.
func (c *CreateParam) UnmarshalJSON(b []byte) error {
	_, _, err := CreateParamJSONScheme.Decode(b, nil, c)
	return err
}

// CreateParamJSONScheme is capable of converting JSON data into the create param Unstructured
// type, which can be used for generic access to objects without a predefined scheme.
var CreateParamJSONScheme runtime.Codec = createParamJSONScheme{}

type createParamJSONScheme struct{}

const createParamJSONSchemeIdentifier runtime.Identifier = "createParamJSON"

func (s createParamJSONScheme) Decode(data []byte, _ *schema.GroupVersionKind, obj runtime.Object) (runtime.Object, *schema.GroupVersionKind, error) {
	var err error
	if obj != nil {
		err = s.decodeInto(data, obj)
	} else {
		obj, err = s.decode(data)
	}

	if err != nil {
		return nil, nil, err
	}

	gvk := obj.GetObjectKind().GroupVersionKind()
	if len(gvk.Kind) == 0 {
		return nil, &gvk, runtime.NewMissingKindErr(string(data))
	}
	return obj, &gvk, nil
}

func (s createParamJSONScheme) Encode(obj runtime.Object, w io.Writer) error {
	if co, ok := obj.(runtime.CacheableObject); ok {
		return co.CacheEncode(s.Identifier(), s.doEncode, w)
	}
	return s.doEncode(obj, w)
}

func (createParamJSONScheme) doEncode(obj runtime.Object, w io.Writer) error {
	switch t := obj.(type) {
	case *CreateParam:
		return json.NewEncoder(w).Encode(t.Object)
	case *runtime.Unknown:
		_, err := w.Write(t.Raw)
		return err
	default:
		return json.NewEncoder(w).Encode(t)
	}
}

// Identifier implements runtime.Encoder interface.
func (createParamJSONScheme) Identifier() runtime.Identifier {
	return createParamJSONSchemeIdentifier
}

func (s createParamJSONScheme) decode(data []byte) (runtime.Object, error) {
	type detector struct {
		Items json.RawMessage
	}
	var det detector
	if err := json.Unmarshal(data, &det); err != nil {
		return nil, err
	}

	//if det.Items != nil {
	//	list := &UnstructuredList{}
	//	err := s.decodeToList(data, list)
	//	return list, err
	//}

	// No Items field, so it wasn't a list.
	unstruct := &CreateParam{}
	err := s.decodeToUnstructured(data, unstruct)
	return unstruct, err
}

func (s createParamJSONScheme) decodeInto(data []byte, obj runtime.Object) error {
	switch x := obj.(type) {
	case *CreateParam:
		return s.decodeToUnstructured(data, x)
	//case *UnstructuredList:
	//	return s.decodeToList(data, x)
	default:
		return json.Unmarshal(data, x)
	}
}

func (createParamJSONScheme) decodeToUnstructured(data []byte, unstruct *CreateParam) error {
	m := make(map[string]interface{})
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}

	unstruct.Object = m

	return nil
}
