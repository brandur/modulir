// Package mtemplatemd provides a template helper function which allows an
// external Markdown file to be included and rendered.
//
// Exists in its own package separate from ftemplate so that it can call into
// mmarkdown in order to offer all the project's standard Markdown niceties.
// mmarkdown calls into ftemplate, so we'd otherwise create a circular
// dependency.
package mtemplatemd

import (
	"context"
	"fmt"
	"html/template"
	"io/ioutil"

	"github.com/brandur/modulir/modules/mmarkdownext"
)

// FuncMap is a set of helper functions to make available in templates for the
// project.
var FuncMap = template.FuncMap{
	"IncludeMarkdown": includeMarkdown,
}

// Using a custom type for context keys is recommended so that they can't be
// interfered with outside the package.
type ftemplateContextKey string

// IncludeMarkdownDependencyKeys is the name of the context key to which
// includeMarkdown will add filenames for included dependencies. This is
// important so that the caller can add them as dependencies to watch for
// rebuilds.
const IncludeMarkdownDependencyKeys = ftemplateContextKey("IncludeMarkdownDependencyKeys")

func includeMarkdown(ctx context.Context, filename string) template.HTML {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		panic(fmt.Sprintf("error rendering Markdown: %s", err))
	}

	if v := ctx.Value(IncludeMarkdownDependencyKeys); v != nil {
		dependencies := v.(map[string]struct{})
		dependencies[filename] = struct{}{}
	}

	s, err := mmarkdownext.Render(string(data), nil)
	if err != nil {
		panic(fmt.Sprintf("error rendering Markdown: %s", err))
	}

	return template.HTML(s)
}
