package golang

import (
	"margo.sh/mg"
)

// Linter wraps mg.Linter to restrict its Langs to Go
//
// all top-level fields are passed along to the underlying Linter
type Linter struct {
	mg.Linter

	Actions []mg.Action

	Name string
	Args []string

	Tag     mg.IssueTag
	Label   string
	TempDir []string
}

// RInit syncs top-level fields with the underlying Linter
func (lt *Linter) RInit(mx *mg.Ctx) {
	l := &lt.Linter
	l.Actions = lt.Actions
	l.Name = lt.Name
	l.Args = lt.Args
	l.Tag = lt.Tag
	l.Label = lt.Label
	l.TempDir = lt.TempDir

	lt.Linter.RInit(mx)
}

// RCond restricts reduction to Go files
func (lt *Linter) RCond(mx *mg.Ctx) bool {
	return mx.LangIs(mg.Go) && lt.Linter.RCond(mx)
}

// GoInstall returns a Linter that runs `go install args...`
func GoInstall(args ...string) *Linter {
	return &Linter{
		Name:  "go",
		Args:  append([]string{"install"}, args...),
		Label: "Go/Install",
	}
}

// GoInstallDiscardBinaries returns a Linter that runs `go install args...`
// it's like GoInstall, but additionally sets GOBIN to a temp directory
// resulting in all binaries being discarded
func GoInstallDiscardBinaries(args ...string) *Linter {
	return &Linter{
		Name:    "go",
		Args:    append([]string{"install"}, args...),
		Label:   "Go/Install",
		TempDir: []string{"GOBIN"},
	}
}

// GoVet returns a Linter that runs `go vet args...`
func GoVet(args ...string) *Linter {
	return &Linter{
		Name:  "go",
		Args:  append([]string{"vet"}, args...),
		Label: "Go/Vet",
	}
}

// GoTest returns a Linter that runs `go test args...`
func GoTest(args ...string) *Linter {
	return &Linter{
		Name:  "go",
		Args:  append([]string{"test"}, args...),
		Label: "Go/Test",
	}
}
