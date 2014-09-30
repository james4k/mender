package mender

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestBuildAndVersionMap(t *testing.T) {
	m, err := Build("testdata/mend.json", "testdata/mend-versions.json", "testdata/_build", os.Stderr)
	if err != nil {
		t.Fatal(err)
	}

	checkResults(t, m)
	m = ParseVersions("testdata/mend-versions.json").(versions).m
	checkResults(t, m)

	err = os.RemoveAll("testdata/_build")
	if err != nil {
		t.Fatal(err)
	}
}

func checkResults(t *testing.T, m map[string]string) {
	if len(m) == 0 {
		t.Fatal("no versioning info")
	}
	for k, v := range m {
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
