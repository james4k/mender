package mender

import (
	"bytes"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
)

type ProcessorFunc func(out io.Writer, in io.Reader) error

var processors = map[string]ProcessorFunc{
	"uglifyjs": func(out io.Writer, in io.Reader) error {
		cmd := exec.Command("uglifyjs")
		cmd.Stdin = in
		cmd.Stdout = out
		cmd.Stderr = os.Stderr
		return cmd.Run()
	},
}

func defaultProcessor(out io.Writer, in io.Reader) error {
	_, err := io.Copy(out, in)
	return err
}

type Spec struct {
	Name string

	// if Pattern is non-empty, we glob match instead of using the Files list.
	Files   []string
	Pattern string

	// a named processor; currently only supports uglifyjs
	Processor string
}

func Process(file, vfile, outputdir string) (map[string]string, error) {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	specs := make(map[string]Spec)
	err = json.Unmarshal(data, &specs)
	if err != nil {
		return nil, err
	}

	dir := filepath.Dir(file)
	os.MkdirAll(outputdir, 0755)

	vmap := make(map[string]string)
	for name, spec := range specs {
		spec.Name = name
		vname, err := ProcessSpec(spec, dir, outputdir)
		if err != nil {
			return nil, err
		}
		vmap[name] = vname
	}

	vdata, err := json.MarshalIndent(vmap, "", "\t")
	if err != nil {
		return nil, err
	}
	vf, err := os.Create(vfile)
	if err != nil {
		return nil, err
	}
	_, err = vf.Write(vdata)
	if err != nil {
		return nil, err
	}
	return vmap, nil
}

func ProcessSpec(spec Spec, dir, outputdir string) (string, error) {
	if len(spec.Pattern) > 0 {
		return ProcessGlob(spec.Name, outputdir, processors[spec.Processor], filepath.Join(dir, spec.Pattern))
	} else {
		for i, f := range spec.Files {
			spec.Files[i] = filepath.Join(dir, f)
		}
		return ProcessFiles(spec.Name, outputdir, processors[spec.Processor], spec.Files...)
	}
	panic("unreachable")
}

func ProcessGlob(name, outputdir string, processor ProcessorFunc, pattern string) (string, error) {
	files, err := filepath.Glob(pattern)
	if err != nil {
		return "", err
	}
	return ProcessFiles(name, outputdir, processor, files...)
}

func ProcessFiles(name, outputdir string, processor ProcessorFunc, files ...string) (string, error) {
	buf := bytes.NewBuffer(make([]byte, 0, 1024))
	hash, err := concatAndHash(buf, files...)
	if err != nil {
		return "", err
	}
	ext := filepath.Ext(name)
	vname := fmt.Sprintf("%s-%x%s", name[:len(name)-len(ext)], hash, ext)
	outputname := filepath.Join(outputdir, vname)
	dir := filepath.Dir(outputname)
	os.MkdirAll(dir, 0755)
	f, err := os.Create(outputname)
	if err != nil {
		return "", err
	}
	if processor == nil {
		processor = defaultProcessor
	}
	err = processor(f, buf)
	if err != nil {
		return "", err
	}
	return vname, nil
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

func VersionMap(vfile string) map[string]string {
	data, err := ioutil.ReadFile(vfile)
	if err != nil {
		panic(err)
	}
	vmap := make(map[string]string)
	err = json.Unmarshal(data, &vmap)
	if err != nil {
		panic(err)
	}
	return vmap
}
