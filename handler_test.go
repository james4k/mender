package mender

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"path"
	"path/filepath"
	"testing"
)

func TestWatchHandler(t *testing.T) {
	h := &Watcher{
		SpecFile: "testdata/mend.json",
		Dir:      "testdata",
	}
	srv := httptest.NewServer(h)
	defer srv.Close()

	h.lazyInit()
	for k := range h.data {
		resp, err := http.Get(srv.URL + "/" + path.Clean(k))
		if err != nil {
			t.Fatal(err)
		}
		actual, err := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
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
