package mfile

import (
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/brandur/modulr/context"
	"github.com/pkg/errors"
)

//
// CopyFile
//

func CopyFile(c *context.Context, source, target string) error {
	if !c.HasChanged(source) {
		return nil
	}

	in, err := os.Open(source)
	if err != nil {
		return errors.Wrap(err, "Error opening copy source")
	}
	defer in.Close()

	out, err := os.Create(target)
	if err != nil {
		return errors.Wrap(err, "Error creating copy target")
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return errors.Wrap(err, "Error copying data")
	}

	return nil
}

func CopyFileToDir(c *context.Context, source, targetDir string) error {
	return CopyFile(c, source, path.Join(targetDir, filepath.Base(source)))
}

//
// EnsureDir
//

func EnsureDir(c *context.Context, target string) error {
	err := os.MkdirAll(target, 0755)
	if err != nil {
		return errors.Wrap(err, "Error creating directory")
	}

	return nil
}

//
//
//

func IsHidden(name) string {
	return strings.HasPrefix(file, ".")
}
