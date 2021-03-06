package utils

import (
	"sort"

	"github.com/pkg/errors"

	"github.com/Masterminds/semver/v3"
)

func GetLatestSemver(vers []string) (string, error) {
	if len(vers) == 0 {
		return "", errors.New("versions length must greater the zero")
	}
	var versions []semver.Version
	for _, item := range vers {
		version, err := semver.NewVersion(item)
		if err != nil {
			continue
		}
		versions = append(versions, *version)
	}
	sort.Slice(versions, func(i, j int) bool {
		return versions[i].GreaterThan(&versions[j])
	})
	return versions[0].String(), nil
}

func GetVersionGreaterThan(v1, v2 string) bool {
	version1, err := semver.NewVersion(v1)
	if err != nil {
		return false
	}
	version2, err := semver.NewVersion(v2)
	if err != nil {
		return false
	}
	return version1.GreaterThan(version2)
}
