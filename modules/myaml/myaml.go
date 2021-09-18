package myaml

import (
	"bytes"
	"errors"
	"io/ioutil"

	"golang.org/x/xerrors"
	"gopkg.in/yaml.v2"

	"github.com/brandur/modulir"
)

// ParseFile is a shortcut from parsing a source file as YAML.
func ParseFile(c *modulir.Context, source string, v interface{}) error {
	raw, err := ioutil.ReadFile(source)
	if err != nil {
		return xerrors.Errorf("error reading file: %w", err)
	}

	err = yaml.Unmarshal(raw, v)
	if err != nil {
		return xerrors.Errorf("error unmarshaling YAML: %w", err)
	}

	c.Log.Debugf("myaml: Parsed file: %s", source)
	return nil
}

// ParseFileFrontmatter is a shortcut from parsing a source file's frontmatter
// (i.e. data at the top between `---` lines) as YAML.
func ParseFileFrontmatter(c *modulir.Context, source string, v interface{}) ([]byte, error) {
	data, err := ioutil.ReadFile(source)
	if err != nil {
		return nil, xerrors.Errorf("error reading file: %w", err)
	}

	frontmatter, content, err := splitFrontmatter(data)
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal(frontmatter, v)
	if err != nil {
		return nil, xerrors.Errorf("error unmarshaling YAML frontmatter: %w", err)
	}

	c.Log.Debugf("myaml: Parsed file frontmatter: %s", source)
	return content, nil
}

//
// Private
//

var errBadFrontmatter = errors.New("error splitting YAML frontmatter")

func splitFrontmatter(data []byte) ([]byte, []byte, error) {
	parts := bytes.Split(data, []byte("---\n"))

	if len(parts) > 1 && !bytes.Equal(parts[0], []byte("")) {
		return nil, nil, errBadFrontmatter
	} else if len(parts) == 2 {
		return nil, bytes.TrimSpace(parts[1]), nil
	} else if len(parts) == 3 {
		return bytes.TrimSpace(parts[1]), bytes.TrimSpace(parts[2]), nil
	}

	return nil, bytes.TrimSpace(parts[0]), nil
}
