package utils

import (
	"fmt"
	"testing"
)

func Test_GetLatestSemver(t *testing.T) {
	var versions = []string{"0.0.1", "0.1.0", "1.0.0", "1.0.0-beta", "1.1.1-beta", "0.0.100"}
	version, _ := GetLatestSemver(versions)
	fmt.Println(version)
}
