package myaml

import (
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"

	"github.com/brandur/modulr/context"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

//
// ParseFileFrontmatter
//

func ParseFileFrontmatter(c *context.Context, source string, data interface{}) ([]byte, bool, error) {
	unchanged := c.IsUnchanged(source)
	if unchanged && !c.Forced() {
		return nil, unchanged, nil
	}

	raw, err := ioutil.ReadFile(source)
	if err != nil {
		return nil, unchanged, errors.Wrap(err, "Error reading file")
	}

	frontmatter, content, err := splitFrontmatter(string(raw))
	if err != nil {
		return nil, unchanged, errors.Wrap(err, "Error splitting frontmatter")
	}


	err = yaml.Unmarshal([]byte(frontmatter), data)
	if err != nil {
		return nil, unchanged, errors.Wrap(err, "Error unmarshaling YAML frontmatter")
	}

	c.Log.Debugf("myaml: Parsed file: %s", source)
	return []byte(content), unchanged, nil
}

//
// Private
//

var errBadFrontmatter = fmt.Errorf("Unable to split YAML frontmatter")

var splitFrontmatterRE = regexp.MustCompile("(?m)^---")

func splitFrontmatter(content string) (string, string, error) {
	parts := splitFrontmatterRE.Split(content, 3)

	if len(parts) > 1 && parts[0] != "" {
		return "", "", errBadFrontmatter
	} else if len(parts) == 2 {
		return "", strings.TrimSpace(parts[1]), nil
	} else if len(parts) == 3 {
		return strings.TrimSpace(parts[1]), strings.TrimSpace(parts[2]), nil
	}

	return "", strings.TrimSpace(parts[0]), nil
}
