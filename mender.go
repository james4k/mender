package mender

import (
	"bytes"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
)

type Spec struct {
	Name string

	// if Pattern is non-empty, we glob match instead of using the Files list.
	Files   []string
	Pattern string
}

func Process(file, outputdir string) error {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}

	specs := make(map[string]Spec)
	err = json.Unmarshal(data, &specs)
	if err != nil {
		return err
	}

	dir := filepath.Dir(file)
	os.MkdirAll(outputdir, 0755)
	for name, spec := range specs {
		spec.Name = name
		err = ProcessSpec(spec, dir, outputdir)
		if err != nil {
			return err
		}
	}
	return nil
}

func ProcessSpec(spec Spec, dir, outputdir string) error {
	if len(spec.Pattern) > 0 {
		return ProcessGlob(spec.Name, outputdir, filepath.Join(dir, spec.Pattern))
	} else {
		for i, f := range spec.Files {
			spec.Files[i] = filepath.Join(dir, f)
		}
		return ProcessFiles(spec.Name, outputdir, spec.Files...)
	}
	panic("unreachable")
}

func ProcessGlob(name, outputdir, pattern string) error {
	files, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}
	return ProcessFiles(name, outputdir, files...)
}

func ProcessFiles(name, outputdir string, files ...string) error {
	buf := bytes.NewBuffer(make([]byte, 0, 2048))
	hash, err := concatAndHash(buf, files...)
	outputname := filepath.Join(outputdir, fmt.Sprintf("%s-%x", name, hash))
	dir := filepath.Dir(outputname)
	os.MkdirAll(dir, 0755)
	f, err := os.Create(outputname)
	if err != nil {
		return err
	}
	_, err = io.Copy(f, buf)
	return err
}

func concatAndHash(dst io.Writer, files ...string) (uint32, error) {
	hash := crc32.NewIEEE()
	for _, name := range files {
		f, err := os.Open(name)
		if err != nil {
			return 0, err
		}
		w := io.MultiWriter(dst, hash)
		_, err = io.Copy(w, f)
		if err != nil {
			return 0, err
		}
	}
	return hash.Sum32(), nil
}
