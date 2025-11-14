package main

import "fmt"

// These variables are set at build time via Bazel stamping.
// See BUILD.bazel in this directory.
var (
	GitCommit string = "(unspecified)"
	GitBranch string = "(unspecified)"
	Version   string = "(unspecified)"
	Date      string = "(unspecified)"
)

// Example function to use the data
func getVersion() string {
	return fmt.Sprintf("%v (commit: %v, branch: %v) built on %v",
		Version, GitCommit, GitBranch, Date)
}
