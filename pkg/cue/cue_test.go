package cue

import (
	"cuelang.org/go/cue"
	"fmt"
	"testing"
)

func Test_CUE(t *testing.T)  {
	const config = `
msg:   "Hello \(place)!"
place: string | *"world" // "world" is the default.
`

	var r cue.Runtime

	instance, _ := r.Compile("test", config)
	inst, _ := instance.Fill("you", "place")
	str, _ := inst.Lookup("msg").String()

	fmt.Println(str)
}
