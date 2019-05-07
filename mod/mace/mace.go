package mace

import (
	"bufio"
	"html/template"
	"os"
	"strings"

	"github.com/brandur/modulr/context"
	"github.com/pkg/errors"
	"github.com/yosssi/ace"
)

func Load(c *context.Context, basePath, innerPath string, opts *ace.Options) (*template.Template, error) {
	if opts == nil {
		opts = &ace.Options{}
	}

	// Ace made a really strange decision to not take extensions when passing
	// around the names of templates, which makes working with known files
	// unnecessarily difficult. Here we correct that by allowing an extension
	// to be passed to the function and stripping it off for Ace's benefit.
 	extlessBasePath := basePath
	if strings.HasSuffix(basePath, ".ace") {
		extlessBasePath = strings.TrimSuffix(basePath, ".ace")
	}
	extlessInnerPath := innerPath
	if strings.HasSuffix(innerPath, ".ace") {
		extlessInnerPath = strings.TrimSuffix(innerPath, ".ace")
	}

	// See the comment above for some context, but since Ace caches templates
	// we always invoke Load and depend on it to take its own fast path if we
	// didn't set DynamicReload.
	template, err := ace.Load(extlessBasePath, extlessInnerPath, opts)
	if err != nil {
		return nil, err
	}

	c.Log.Debugf("mace: Loaded template layout '%s' view '%s'",
		basePath, innerPath)

	return template, nil
}

// Load2 loads a template.
//
// TODO: Get rid of this.
func Load2(c *context.Context, basePath, innerPath string, opts *ace.Options) (*template.Template, bool, error) {
	if opts == nil {
		opts = &ace.Options{}
	}

	aceBasePath := basePath
	if !strings.HasSuffix(basePath, ".ace") {
		aceBasePath = basePath + ".ace"
	}
	aceInnerPath := innerPath
	if !strings.HasSuffix(innerPath, ".ace") {
		aceInnerPath = innerPath + ".ace"
	}

	// TODO: Check existence of files.

	changedBasePath := c.Changed(aceBasePath)
	changedInnerPath := c.Changed(aceInnerPath)

	// By default Ace will use a built-in caching mechanism and only load any
	// given template one time.
	//
	// If we detect that the source template or view files have changed then
	// set the special DynamicReload option to force the template to reload.
	if changedBasePath || changedInnerPath || c.Forced() {
		opts.DynamicReload = true
	}

 	extlessBasePath := basePath
	if strings.HasSuffix(basePath, ".ace") {
		extlessBasePath = strings.TrimSuffix(basePath, ".ace")
	}
	extlessInnerPath := innerPath
	if strings.HasSuffix(innerPath, ".ace") {
		extlessInnerPath = strings.TrimSuffix(innerPath, ".ace")
	}

	c.Log.Debugf("mace: Loaded template layout '%s' view '%s'",
		basePath, innerPath)

	// See the comment above for some context, but since Ace caches templates
	// we always invoke Load and depend on it to take its own fast path if we
	// didn't set DynamicReload.
	template, err := ace.Load(extlessBasePath, extlessInnerPath, opts)

	return template, changedBasePath || changedInnerPath, err
}

// Render is a shortcut for loading an Ace template and rendering it to a
// target file.
func Render(c *context.Context, basePath, innerPath, target string,
	opts *ace.Options, locals map[string]interface{}) error {

	template, err := Load(c, basePath, innerPath, opts)
	if err != nil {
		return errors.Wrap(err, "Error loading template")
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

	c.Log.Debugf("mace: Rendered view '%s' to '%s'",
		innerPath, target)

	return nil
}

// Render2 is a shortcut for loading an Ace template and rendering it to a
// target file.
//
// Note that the values in locals are not considered when determining whether
// the sources are unchanged, so make sure to use a forced context in case a
// change is made to them.
//
// TODO: Get rid of this.
func Render2(c *context.Context, basePath, innerPath, target string,
	opts *ace.Options, locals map[string]interface{}) (bool, error) {

	template, changed, err := Load2(c, basePath, innerPath, opts)
	if err != nil {
		return changed, errors.Wrap(err, "Error loading template")
	}

	if !changed && !c.Forced() {
		return false, nil
	}

	file, err := os.Create(target)
	if err != nil {
		return changed, errors.Wrap(err, "Error creating target file")
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	err = template.Execute(writer, locals)
	if err != nil {
		return changed, errors.Wrap(err, "Error rendering template")
	}

	c.Log.Debugf("mace: Rendered view '%s' to '%s'",
		innerPath, target)
	return changed, nil
}
