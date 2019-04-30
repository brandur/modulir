package mmarkdown

import (
	"io/ioutil"

	"github.com/brandur/modulr/context"
	"github.com/brandur/modulr/mod/mfile"
	"github.com/pkg/errors"
	"gopkg.in/russross/blackfriday.v2"
)

func Render(c *context.Context, data []byte) []byte {
	return blackfriday.Run(data)
}

func RenderFile(c *context.Context, source, target string) error {
	inData, unchanged, err := mfile.ReadFile(c, source)
	if err != nil {
		return errors.Wrap(err, "Error rendering file")
	}
	if unchanged && !c.Forced() {
		return nil
	}

	outData := Render(c, inData)

	err = ioutil.WriteFile(target, outData, 0644)
	if err != nil {
		return errors.Wrap(err, "Error writing file")
	}

	c.Log.Debugf("mmarkdown: Rendered '%s' to '%s'", source, target)
	return nil
}

