package mmarkdown

import (
	"io/ioutil"

	"github.com/brandur/modulir"
	"github.com/pkg/errors"
	"gopkg.in/russross/blackfriday.v2"
)

// Render is a shortcut for rendering some source data to Markdown via Black
// Friday.
func Render(c *modulir.Context, data []byte) []byte {
	return blackfriday.Run(data)
}

// RenderFile is a shortcut for rendering a source file to Markdown in a target
// file via Black Friday.
func RenderFile(c *modulir.Context, source, target string) error {
	inData, err := ioutil.ReadFile(source)
	if err != nil {
		return errors.Wrap(err, "Error reading file")
	}

	outData := Render(c, inData)

	err = ioutil.WriteFile(target, outData, 0644)
	if err != nil {
		return errors.Wrap(err, "Error writing file")
	}

	c.Log.Debugf("mmarkdown: Rendered '%s' to '%s'", source, target)
	return nil
}
