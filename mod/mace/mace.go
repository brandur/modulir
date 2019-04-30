package mace

import (
	"bufio"
	"html/template"
	"os"

	"github.com/brandur/modulr/context"
	"github.com/pkg/errors"
	"github.com/yosssi/ace"
)

func Load(c *context.Context, basePath, innerPath string, opts *ace.Options) (*template.Template, bool, error) {
	if opts == nil {
		opts = &ace.Options{}
	}

	aceBasePath := basePath + ".ace"
	aceInnerPath := innerPath + ".ace"

	// TODO: Check existence of files.

	unchangedBasePath := c.IsUnchanged(aceBasePath)
	unchangedInnerPath := c.IsUnchanged(aceInnerPath)

	// By default Ace will use a built-in caching mechanism and only load any
	// given template one time.
	//
	// If we detect that the source template or view files have changed then
	// set the special DynamicReload option to force the template to reload.
	if !unchangedBasePath || !unchangedInnerPath || c.Forced() {
		opts.DynamicReload = true
	}

	c.Log.Debugf("mace: Loaded template: layout '%s' view '%s'",
		basePath, innerPath)

	// See the comment above for some context, but since Ace caches templates
	// we always invoke Load and depend on it to take its own fast path if we
	// didn't set DynamicReload.
	template, err := ace.Load(basePath, innerPath, opts)

	return template, unchangedBasePath && unchangedInnerPath, err
}

// Render is a shortcut for loading an Ace template and rendering it to a
// target file.
//
// Note that the values in locals are not considered when determining whether
// the sources are unchanged, so make sure to use a forced context in case a
// change is made to them.
func Render(c *context.Context, basePath, innerPath, target string,
		opts *ace.Options, locals map[string]interface{}) error {

	template, unchanged, err := Load(c, basePath, innerPath, opts)
	if err != nil {
		return errors.Wrap(err, "Error loading template")
	}

	if unchanged && !c.Forced() {
		return nil
	}

	file, err := os.Create(target)
	if err != nil {
		return errors.Wrap(err, "Error creating target file")
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	err = template.Execute(writer, locals)
	if err != nil {
		return errors.Wrap(err, "Error rendering template")
	}

	return nil
}
