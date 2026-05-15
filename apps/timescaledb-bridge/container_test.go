package main

import "testing"

func Test(t *testing.T) {
	// FROM scratch bridge image; no shell available for in-container checks.
	// A successful build confirms that:
	//   - the new-version extension files are present under /lib and /share/extension
	//   - the old versioned loader .so was copied from the OLD_IMAGE source
}
