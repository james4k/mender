package mender

import (
	"code.google.com/p/go.exp/fsnotify"
	"io"
	"io/ioutil"
	"log"
	"mime"
	"net/http"
	"path/filepath"
	"sync"
	"time"
)

type LiveHandler struct {
	sync.RWMutex
	specfile string
	dir      string
	fallback http.Handler
	vmap     map[string]string
	data     map[string][]byte
	stderr   io.Writer
	logger   *log.Logger
	OnChange func() // called when new changes are found on disk
}

// LiveServer returns an HTTP handler which watches for changes and serves the
// processed assets from memory. Nothing is written to disk. This is intended
// for use during development, and niceties such as a cache of old versions are
// not provided. Provide a Writer to get stderr output from the processors.
func LiveServer(specfile, dir string, fallback http.Handler, stderr io.Writer) *LiveHandler {
	if stderr == nil {
		stderr = ioutil.Discard
	}
	h := &LiveHandler{
		specfile: specfile,
		dir:      dir,
		fallback: fallback,
		stderr:   stderr,
		logger:   log.New(stderr, "", log.LstdFlags),
	}
	h.mend()
	go h.watch()
	time.AfterFunc(time.Second/10, func() {
		h.OnChange()
	})
	return h
}

func (h *LiveHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	h.RLock()
	for k, v := range h.vmap {
		if req.URL.Path == "/"+v {
			w.Header().Set("Content-Type", mime.TypeByExtension(filepath.Ext(k)))
			w.Write(h.data[k])
			h.RUnlock()
			return
		}
	}
	h.RUnlock()
	if h.fallback != nil {
		h.fallback.ServeHTTP(w, req)
	}
}

// VersionMap returns the latest versioned asset names. Useful from LiveHandler.OnChange
func (h *LiveHandler) VersionMap() map[string]string {
	h.Lock()
	defer h.Unlock()
	return h.vmap
}

func (h *LiveHandler) log(s interface{}) {
	h.logger.Println(s)
}

func (h *LiveHandler) watch() {
	specs, err := ReadSpecs(h.specfile)
	if err != nil {
		h.log(err)
		return
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		h.log(err)
		return
	}
	defer watcher.Close()

	watcher.Watch(h.specfile)
	for _, s := range specs {
		for _, f := range s.Files {
			watcher.Watch(filepath.Join(h.dir, f))
		}
	}

	select {
	case <-watcher.Event:
	case err := <-watcher.Error:
		h.log(err)
		return
	}
	time.Sleep(time.Second / 10)

	h.mend()
	h.OnChange()
	go h.watch()
}

func (h *LiveHandler) mend() {
	specs, err := ReadSpecs(h.specfile)
	if err != nil {
		h.log(err)
		return
	}
	vmap := make(map[string]string, len(specs))
	data := make(map[string][]byte, len(specs))
	for _, s := range specs {
		vname, d, err := ProcessSpec(s, h.dir, h.stderr)
		if err != nil {
			h.log(err)
			return
		}
		vmap[s.Name] = vname
		data[s.Name] = d
	}
	h.Lock()
	h.vmap = vmap
	h.data = data
	h.Unlock()
}
