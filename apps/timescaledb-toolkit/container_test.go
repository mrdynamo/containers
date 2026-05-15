package main

import (
	"testing"

	"github.com/home-operations/containers/testhelpers"
)

func Test(t *testing.T) {
	// This is a FROM scratch artifact image containing only the compiled
	// timescaledb_toolkit extension files. There is no shell or OS layer,
	// so in-container filesystem checks are not possible. A successful build
	// (which copies the .control file into /share/extension/) is the
	// functional validation.
	_ = testhelpers.GetTestImage("ghcr.io/mrdynamo/timescaledb-toolkit:rolling")
}
