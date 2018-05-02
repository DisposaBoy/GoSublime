package mgutil

// NoCopy is used to prevent struct copying through `go vet`s -copylocks checker.
// It's an export of the type `sync.noCopy`
// See https://golang.org/issues/8005#issuecomment-190753527
//
// To prevent struct copying, add a field `noCopy NoCopy` to the struct
type NoCopy struct{}

// Lock is a no-op used by the `go vet -copylocks` checker.
func (*NoCopy) Lock() {}
