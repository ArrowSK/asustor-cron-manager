package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type entry struct {
	name string
	mode int64
	body []byte
}

func main() {
	root := flag.String("root", "", "staged package root")
	out := flag.String("out", "", "output .apk file")
	version := flag.String("version", "0.0.0", "package version")
	flag.Parse()
	if *root == "" || *out == "" {
		flag.Usage()
		os.Exit(2)
	}
	if err := build(*root, *out, *version); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func build(root, out, version string) error {
	control, data, err := collect(root, version)
	if err != nil {
		return err
	}
	if len(control) == 0 {
		return fmt.Errorf("CONTROL is empty")
	}
	cg, err := tarGz(control)
	if err != nil {
		return err
	}
	dg, err := tarGz(data)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	add := func(name string, b []byte) error {
		h := &zip.FileHeader{Name: name, Method: zip.Store}
		w, err := zw.CreateHeader(h)
		if err != nil {
			return err
		}
		_, err = w.Write(b)
		return err
	}
	if err = add("apkg-version", []byte("2.0\n")); err != nil {
		return err
	}
	if err = add("control.tar.gz", cg); err != nil {
		return err
	}
	if err = add("data.tar.gz", dg); err != nil {
		return err
	}
	return zw.Close()
}

func collect(root, version string) (control, data []entry, err error) {
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		rel, e := filepath.Rel(root, path)
		if e != nil {
			return e
		}
		rel = filepath.ToSlash(rel)
		b, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		mode := int64(0o644)
		base := filepath.Base(path)
		if strings.HasSuffix(base, ".sh") || base == "cron-manager" {
			mode = 0o755
		}
		if strings.HasSuffix(base, ".sh") {
			b = bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n"))
			b = bytes.ReplaceAll(b, []byte("\r"), []byte("\n"))
		}
		if strings.HasPrefix(rel, "CONTROL/") {
			name := strings.TrimPrefix(rel, "CONTROL/")
			if name == "config.json" {
				b = bytes.ReplaceAll(b, []byte("APKG_VERSION"), []byte(version))
			}
			control = append(control, entry{name, mode, b})
		} else {
			data = append(data, entry{rel, mode, b})
		}
		return nil
	})
	sort.Slice(control, func(i, j int) bool { return control[i].name < control[j].name })
	sort.Slice(data, func(i, j int) bool { return data[i].name < data[j].name })
	return
}

func tarGz(es []entry) ([]byte, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	stamp := time.Unix(0, 0).UTC()
	for _, e := range es {
		h := &tar.Header{Name: "./" + e.name, Mode: e.mode, Size: int64(len(e.body)), ModTime: stamp, Typeflag: tar.TypeReg, Uname: "root", Gname: "root"}
		if err := tw.WriteHeader(h); err != nil {
			return nil, err
		}
		if _, err := tw.Write(e.body); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
