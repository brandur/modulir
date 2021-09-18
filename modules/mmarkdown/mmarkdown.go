package mmarkdown

import (
	"io/ioutil"

	"golang.org/x/xerrors"
	"gopkg.in/russross/blackfriday.v2"

	"github.com/brandur/modulir"
)

//////////////////////////////////////////////////////////////////////////////
//
//
//
// Public
//
//
//
//////////////////////////////////////////////////////////////////////////////

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
		return xerrors.Errorf("error reading file: %w", err)
	}

	outData := Render(c, inData)

	err = ioutil.WriteFile(target, outData, 0o600)
	if err != nil {
		return xerrors.Errorf("error writing file: %w", err)
	}

	c.Log.Debugf("mmarkdown: Rendered '%s' to '%s'", source, target)
	return nil
}
