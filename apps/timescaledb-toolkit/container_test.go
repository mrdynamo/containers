package main

import (
	"context"
	"testing"

	"github.com/home-operations/containers/testhelpers"
)

func Test(t *testing.T) {
	ctx := context.Background()
	image := testhelpers.GetTestImage("ghcr.io/mrdynamo/timescaledb-toolkit:rolling")
	testhelpers.TestFileExists(t, ctx, image, "/out/share/extension/timescaledb_toolkit.control", nil)
}
