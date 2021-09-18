package mtoml

import (
	"bytes"
	"errors"
	"io/ioutil"

	"github.com/brandur/modulir"
	"github.com/pelletier/go-toml"
	"golang.org/x/xerrors"
)

// ParseFile is a shortcut from parsing a source file as TOML.
func ParseFile(c *modulir.Context, source string, v interface{}) error {
	data, err := ioutil.ReadFile(source)
	if err != nil {
		return xerrors.Errorf("error reading file: %w", err)
	}

	err = toml.Unmarshal(data, v)
	if err != nil {
		return xerrors.Errorf("error unmarshaling TOML: %w", err)
	}

	c.Log.Debugf("mtoml: Parsed file: %s", source)
	return nil
}

// ParseFileFrontmatter is a shortcut from parsing a source file's frontmatter
// (i.e. data at the top between `+++` lines) as TOML.
func ParseFileFrontmatter(c *modulir.Context, source string, v interface{}) ([]byte, error) {
	data, err := ioutil.ReadFile(source)
	if err != nil {
		return nil, xerrors.Errorf("error reading file: %w", err)
	}

	frontmatter, content, err := splitFrontmatter(data)
	if err != nil {
		return nil, err
	}

	err = toml.Unmarshal(frontmatter, v)
	if err != nil {
		return nil, xerrors.Errorf("error unmarshaling TOML frontmatter: %w", err)
	}

	c.Log.Debugf("mtoml: Parsed file frontmatter: %s", source)
	return content, nil
}

//
// Private
//

var errBadFrontmatter = errors.New("error splitting TOML frontmatter")

func splitFrontmatter(data []byte) ([]byte, []byte, error) {
	parts := bytes.Split(data, []byte("+++\n"))

	if len(parts) > 1 && !bytes.Equal(parts[0], []byte("")) {
		return nil, nil, errBadFrontmatter
	} else if len(parts) == 2 {
		return nil, bytes.TrimSpace(parts[1]), nil
	} else if len(parts) == 3 {
		return bytes.TrimSpace(parts[1]), bytes.TrimSpace(parts[2]), nil
	}

	return nil, bytes.TrimSpace(parts[0]), nil
}
