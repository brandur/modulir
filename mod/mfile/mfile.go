package mfile

import (
	"io"
	"io/ioutil"
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
	unchanged := c.IsUnchanged(source)
	if unchanged && !c.Forced() {
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

	c.Log.Debugf("mfile: Copied '%s' to '%s'", source, target)
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

	c.Log.Debugf("mfile: Ensured dir existence: %s", target)
	return nil
}

//
// IsHidden
//

func IsHidden(source string) bool {
	file := filepath.Base(source)
	return strings.HasPrefix(file, ".")
}

//
// ReadFile
//

func ReadFile(c *context.Context, source string) ([]byte, bool, error) {
	unchanged := c.IsUnchanged(source)
	if unchanged && !c.Forced() {
		return nil, unchanged, nil
	}

	in, err := os.Open(source)
	if err != nil {
		return nil, unchanged, errors.Wrap(err, "Error opening read source")
	}

	data, err := ioutil.ReadAll(in)
	if err != nil {
		return nil, unchanged, errors.Wrap(err, "Error reading source")
	}

	c.Log.Debugf("mfile: Read file: %s", source)
	return data, unchanged, nil
}

//
// ReadDir
//

// ReadDir reads files in a directory and returns a list of file paths.
//
// Unlike ioutil.ReadDir, this function skips hidden files, returns a list of
// full paths (easier to plumb into other functions), and sets up a watch on
// the listed source.
func ReadDir(c *context.Context, source string) ([]string, error) {
	infos, err := ioutil.ReadDir(source)
	if err != nil {
		return nil, errors.Wrap(err, "Error reading directory")
	}

	var files []string

	for _, info := range infos {
		if IsHidden(info.Name()) {
			continue
		}

		files = append(files, path.Join(source, info.Name()))
	}

	c.Log.Debugf("mfile: Read dir: %s", source)
	return files, nil
}
