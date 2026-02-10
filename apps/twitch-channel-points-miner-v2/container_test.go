package main

import (
	"context"
	"testing"

	"github.com/home-operations/containers/testhelpers"
)

func Test(t *testing.T) {
	ctx := context.Background()
	image := testhelpers.GetTestImage("ghcr.io/mrdynamo/twitch-channel-points-miner-v2:rolling")
	testhelpers.TestFileExists(t, ctx, image, "/app/run.py", nil)
}

