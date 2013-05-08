package mender

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestBuildAndVersionMap(t *testing.T) {
	vmap, err := Build("testdata/mend.json", "testdata/mend-versions.json", "testdata/_build", os.Stderr)
	if err != nil {
		t.Fatal(err)
	}

	checkResults(t, vmap)
	vmap = VersionMap("testdata/mend-versions.json")
	checkResults(t, vmap)

	err = os.RemoveAll("testdata/_build")
	if err != nil {
		t.Fatal(err)
	}
}

func checkResults(t *testing.T, vmap map[string]string) {
	if len(vmap) == 0 {
		t.Fatal("no versioning info")
	}
	for k, v := range vmap {
		actual, err := ioutil.ReadFile(filepath.Join("testdata/_build", v))
		if err != nil {
			t.Fatal(err)
		}
		expected, err := ioutil.ReadFile(filepath.Join("testdata/_expected", k))
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(actual, expected) {
			t.Fatal("expected did not match actual")
		}
	}
}
