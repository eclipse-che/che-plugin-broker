//
// Copyright (c) 2012-2018 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package files

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type IoUtil interface {
	Download(URL string, destPath string) error
	CopyResource(src string, dest string) error
	CopyFile(src string, dest string) error
	ResolveDestPath(filePath string, destDir string) string
	ResolveDestPathFromURL(url string, destDir string) string
	TempDir(string, string) (string, error)
	Unzip(arch string, dest string) error
	Untar(tarPath string, dest string) error
	CreateFile(file string, tr io.Reader) error
	ClearDir(dir string) error
}

type impl struct{}

func New() IoUtil {
	return &impl{}
}

func (util *impl) Download(URL string, destPath string) error {
	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	resp, err := http.Get(URL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	return nil
}

func (util *impl) TempDir(baseDir string, prefix string) (dirPath string, err error) {
	return ioutil.TempDir(baseDir, prefix)
}

func (util *impl) CopyResource(src string, dest string) error {
	cmd := exec.Command("cp", "-r", src, dest)
	return cmd.Run()
}

func (util *impl) CopyFile(src string, dest string) error {
	to, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer to.Close()

	from, err := os.Open(src)
	if err != nil {
		return err
	}
	defer from.Close()

	_, err = io.Copy(to, from)
	return err
}

func (util *impl) ResolveDestPath(filePath string, destDir string) string {
	destName := filepath.Base(filePath)
	destPath := filepath.Join(destDir, destName)
	return destPath
}

func (util *impl) ResolveDestPathFromURL(url string, destDir string) string {
	tokens := strings.Split(url, "/")
	fileName := tokens[len(tokens)-1]
	destPath := filepath.Join(destDir, fileName)
	return destPath
}

func (util *impl) Unzip(arch string, dest string) error {
	r, err := zip.OpenReader(arch)
	if err != nil {
		return err
	}
	defer func() {
		if err := r.Close(); err != nil {
			panic(err)
		}
	}()

	if err := os.MkdirAll(dest, 0755); err != nil {
		return err
	}

	// Closure to address file descriptors issue with all the deferred .Close() methods
	extractAndWriteFile := func(f *zip.File) error {
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer func() {
			if err := rc.Close(); err != nil {
				panic(err)
			}
		}()

		path := filepath.Join(dest, f.Name)

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(path, 0755); err != nil {
				return err
			}
		} else {
			if err := os.MkdirAll(filepath.Dir(path), 0775); err != nil {
				return err
			}
			f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return err
			}
			defer func() {
				if err := f.Close(); err != nil {
					panic(err)
				}
			}()

			_, err = io.Copy(f, rc)
			if err != nil {
				return err
			}
		}
		return nil
	}

	for _, f := range r.File {
		err := extractAndWriteFile(f)
		if err != nil {
			return err
		}
	}

	return nil
}

func (util *impl) Untar(tarPath string, dest string) error {
	file, err := os.Open(tarPath)
	if err != nil {
		return err
	}
	gzr, err := gzip.NewReader(bufio.NewReader(file))
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()

		switch {
		// no more files, unpacking is finished
		case err == io.EOF:
			return nil

		case err != nil:
			return err

			// skip empty header
		case header == nil:
			continue
		}

		tarEntry := filepath.Join(dest, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(tarEntry, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := util.createContainingDir(tarEntry); err != nil {
				return err
			}
			if err := util.CreateFile(tarEntry, tr); err != nil {
				return err
			}
		default:
			log.Printf("Unexpected entry in tar archive is skipped. Type: %x Path: %s", header.Typeflag, tarEntry)
		}
	}
}

func (util *impl) createContainingDir(filePath string) error {
	dirPath := filepath.Dir(filePath)
	return os.MkdirAll(dirPath, 0755)
}

func (util *impl) CreateFile(file string, tr io.Reader) error {
	f, err := os.Create(file)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(f, tr); err != nil {
		return err
	}
	return f.Sync()
}

func (util *impl) ClearDir(dir string) error {
	files, err := filepath.Glob(filepath.Join(dir, "*"))
	if err != nil {
		return err
	}

	for _, file := range files {
		err = os.RemoveAll(file)
		if err != nil {
			return err
		}
	}
	return nil
}
