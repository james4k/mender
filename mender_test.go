package mender

import (
	"testing"
)

func TestProcess(t *testing.T) {
	// TODO: make some basic test inputs

	err := Process("testdata/mend.json", "testdata/_build")
	if err != nil {
		t.Fatal(err)
	}

	// TODO: verify the output
}
