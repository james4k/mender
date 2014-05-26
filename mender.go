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
	"strings"
)

type ProcessorFunc func(out io.Writer, in io.Reader, werr io.Writer) error

type processor struct {
	Func     ProcessorFunc
	LiveMode bool // if true, we'll run this processor in live mode too
}

var processors = map[string]processor{
	"uglifyjs": {
		Func: func(out io.Writer, in io.Reader, werr io.Writer) error {
			cmd := exec.Command("uglifyjs")
			cmd.Stdin = in
			cmd.Stdout = out
			cmd.Stderr = werr
			return cmd.Run()
		},
	},
	// TODO: Probably doesn't work...need to change the processor api to allow
	// assembling the files themselves, as the input file extensions have
	// meaning to tsc, so a concatenated jumble of javascript is kind of
	// useless. Oh, and needs to be passed only the .ts and .d.ts files.
	"typescript FIXME": {
		Func: func(out io.Writer, in io.Reader, werr io.Writer) error {
			// god dammit typescript. why don't you use stdin/out!??
			dir, err := ioutil.TempDir(os.TempDir(), "mender")
			if err != nil {
				return err
			}
			defer os.RemoveAll(dir)
			fin, err := os.Create(filepath.Join(dir, "in.ts"))
			if err != nil {
				return err
			}
			_, err = io.Copy(fin, in)
			if err != nil {
				fin.Close()
				return err
			}
			err = fin.Close()
			if err != nil {
				return err
			}
			cmd := exec.Command("tsc", "--comments", "--out", filepath.Join(dir, "out"), filepath.Join(dir, "in.ts"))
			cmd.Stderr = werr
			err = cmd.Run()
			if err != nil {
				return err
			}
			f, err := os.Open(filepath.Join(dir, "out"))
			if err != nil {
				return err
			}
			defer f.Close()
			_, err = io.Copy(out, f)
			return err
		},
		LiveMode: true,
	},
}

func defaultProcessor(out io.Writer, in io.Reader, werr io.Writer) error {
	_, err := io.Copy(out, in)
	return err
}

func composeProcessors(names string) ProcessorFunc {
	var pp []processor
	s := strings.Fields(names)
	for i := len(s) - 1; i >= 0; i-- {
		p, ok := processors[s[i]]
		if ok {
			pp = append(pp, p)
		}
	}
	if true {
		return func(out io.Writer, in io.Reader, werr io.Writer) error {
			// Lots of copying, but should be negligible. Maybe in the future we
			// can use pipes and run the asset processors in parallel.
			buf := bytes.NewBuffer(make([]byte, 0, 1024))
			copybuf := bytes.NewBuffer(make([]byte, 0, 1024))
			for i := 0; i < len(pp)-1; i++ {
				err := pp[i].Func(buf, in, werr)
				if err != nil {
					return err
				}
				in = buf
				copybuf.Reset()
				_, err = io.Copy(copybuf, buf)
				if err != nil {
					return err
				}
			}
			_, err := io.Copy(out, in)
			return err
		}
	}
	// the below func deadlocks...may need to be buffered after all
	return func(out io.Writer, in io.Reader, werr io.Writer) error {
		errc := make(chan error)
		pr, pw := io.Pipe()
		for i := 0; i < len(pp)-1; i++ {
			go func() {
				err := pp[i].Func(pw, in, werr)
				if err != nil {
					errc <- err
				}
			}()
			in = pr
			pr, pw = io.Pipe()
		}
		go func() {
			_, err := io.Copy(out, in)
			errc <- err
		}()
		return <-errc
	}
}

type Spec struct {
	Name string

	// if Pattern is non-empty, we glob match instead of using the Files list.
	Files   []string
	Pattern string

	// named processors to run; currently only supports uglifyjs
	Processors string
}

func ReadSpecs(file string) (map[string]*Spec, error) {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	specs := make(map[string]*Spec)
	err = json.Unmarshal(data, &specs)
	if err != nil {
		return nil, err
	}
	for name, spec := range specs {
		spec.Name = name
	}
	return specs, nil
}

// Build processes each spec and writes to disk the built files and version spec file.
func Build(file, vfile, outputdir string, errw io.Writer) (map[string]string, error) {
	specs, err := ReadSpecs(file)
	if err != nil {
		return nil, err
	}
	dir := filepath.Dir(file)
	os.MkdirAll(outputdir, 0755)

	vmap := make(map[string]string)
	for name, spec := range specs {
		vname, data, err := ProcessSpec(spec, dir, errw)
		if err != nil {
			return nil, err
		}
		outputname := filepath.Join(outputdir, vname)
		dir := filepath.Dir(outputname)
		os.MkdirAll(dir, 0755)
		f, err := os.Create(outputname)
		if err != nil {
			return nil, err
		}
		f.Write(data)
		f.Close()
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

func ProcessSpec(spec *Spec, dir string, errw io.Writer) (string, []byte, error) {
	if len(spec.Pattern) > 0 {
		return ProcessGlob(spec.Name, composeProcessors(spec.Processors), errw, filepath.Join(dir, spec.Pattern))
	} else {
		for i, f := range spec.Files {
			spec.Files[i] = filepath.Join(dir, f)
		}
		return ProcessFiles(spec.Name, composeProcessors(spec.Processors), errw, spec.Files...)
	}
	panic("unreachable")
}

func ProcessGlob(name string, processor ProcessorFunc, errw io.Writer, pattern string) (string, []byte, error) {
	files, err := filepath.Glob(pattern)
	if err != nil {
		return "", nil, err
	}
	return ProcessFiles(name, processor, errw, files...)
}

func ProcessFiles(name string, processor ProcessorFunc, errw io.Writer, files ...string) (string, []byte, error) {
	buf := bytes.NewBuffer(make([]byte, 0, 1024))
	hash, err := concatAndHash(buf, files...)
	if err != nil {
		return "", nil, err
	}
	ext := filepath.Ext(name)
	vname := fmt.Sprintf("%s-%x%s", name[:len(name)-len(ext)], hash, ext)
	if processor == nil {
		processor = defaultProcessor
	}
	outbuf := bytes.NewBuffer(make([]byte, 0, 1024))
	if errw == nil {
		errw = ioutil.Discard
	}
	err = processor(outbuf, buf, errw)
	if err != nil {
		return "", nil, err
	}
	return vname, outbuf.Bytes(), nil
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
		// file probably doesn't exist, so just give zero val
		return nil
	}
	vmap := make(map[string]string)
	err = json.Unmarshal(data, &vmap)
	if err != nil {
		// panics used as this is func is commonly called for package-scope vars,
		// and there was something wrong with the json formatting...
		panic(err)
	}
	return vmap
}
