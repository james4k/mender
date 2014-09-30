// +build !appengine

package mender

import (
	"io"
	"io/ioutil"
	"log"
	"mime"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"code.google.com/p/go.exp/fsnotify"
)

// Watcher is an HTTP handler which watches for changes and serves the
// processed assets from memory. Nothing is written to disk. This is
// intended for use during development, and niceties such as a cache of
// old versions are not provided. Provide a Writer to get stderr output
// from the processors.
type Watcher struct {
	mu     sync.RWMutex
	data   map[string][]byte
	logger *log.Logger

	SpecFile  string
	Dir       string
	Fallback  http.Handler
	LogWriter io.Writer
}

func (w *Watcher) lazyInit() {
	if w.logger != nil {
		return
	}
	if w.LogWriter == nil {
		w.LogWriter = ioutil.Discard
	}
	w.logger = log.New(w.LogWriter, "", log.LstdFlags)
	w.mend()
	go w.watch()
}

func (w *Watcher) Version(s string) string {
	w.lazyInit()
	return s
}

func (w *Watcher) ServeHTTP(writer http.ResponseWriter, req *http.Request) {
	w.lazyInit()
	w.mu.RLock()
	k := req.URL.Path[1:]
	v, ok := w.data[k]
	if ok {
		writer.Header().Set("Content-Type", mime.TypeByExtension(filepath.Ext(k)))
		writer.Write(v)
		w.mu.RUnlock()
		return
	}
	w.mu.RUnlock()
	if w.Fallback != nil {
		w.Fallback.ServeHTTP(writer, req)
	}
}

func (w *Watcher) log(s interface{}) {
	w.logger.Println(s)
}

func (w *Watcher) watch() {
	specs, err := ReadSpecs(w.SpecFile)
	if err != nil {
		w.log(err)
		return
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		w.log(err)
		return
	}
	defer watcher.Close()
	watcher.Watch(w.SpecFile)
	for _, s := range specs {
		for _, f := range s.Files {
			watcher.Watch(filepath.Join(w.Dir, f))
		}
	}
	select {
	case <-watcher.Event:
	case err := <-watcher.Error:
		w.log(err)
		return
	}
	time.Sleep(100 * time.Millisecond)
	w.mend()
	go w.watch()
}

func (w *Watcher) mend() {
	specs, err := ReadSpecs(w.SpecFile)
	if err != nil {
		w.log(err)
		return
	}
	data := make(map[string][]byte, len(specs))
	for _, s := range specs {
		_, d, err := ProcessSpec(s, w.Dir, w.LogWriter)
		if err != nil {
			w.log(err)
			return
		}
		data[s.Name] = d
	}
	w.mu.Lock()
	w.data = data
	w.mu.Unlock()
}
