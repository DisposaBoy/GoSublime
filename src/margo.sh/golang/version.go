package golang

import (
	"go/build"
	"strconv"
	"strings"
)

var (
	VersionTag string         = build.Default.ReleaseTags[len(build.Default.ReleaseTags)-1]
	Version    ReleaseVersion = func() ReleaseVersion {
		s := strings.TrimPrefix(VersionTag, "go")
		l := strings.SplitN(s, ".", 2)
		v := ReleaseVersion{}
		v.Major, _ = strconv.Atoi(l[0])
		v.Minor, _ = strconv.Atoi(l[1])
		return v
	}()
)

type ReleaseVersion struct {
	Major int
	Minor int
}
