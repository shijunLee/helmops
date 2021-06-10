package actions

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func Test_RepoPath(t *testing.T) {
	repoFile := "/helm/config/repositories.yaml"
	var repopath = strings.Replace(repoFile, filepath.Ext(repoFile), ".lock", 1)
	fmt.Println(repopath)
}
