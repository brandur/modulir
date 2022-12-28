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
	"os"

	"github.com/brandur/modulir/modules/mmarkdownext"
)

// FuncMap is a set of helper functions to make available in templates for the
// project.
var FuncMap = template.FuncMap{
	"IncludeMarkdown": IncludeMarkdown,
}

// ContextKey is the name of the context key to which IncludeMarkdown will add
// filenames for included dependencies. This is important so that the caller can
// add them as dependencies to watch for rebuilds.
type ContextKey struct{}

type ContextContainer struct {
	Dependencies    []string
	dependenciesMap map[string]struct{}
}

func Context(ctx context.Context) (context.Context, *ContextContainer) {
	container := &ContextContainer{dependenciesMap: make(map[string]struct{})}
	return context.WithValue(ctx, ContextKey{}, container), container
}

func IncludeMarkdown(ctx context.Context, filename string) template.HTML {
	data, err := os.ReadFile(filename)
	if err != nil {
		panic(fmt.Sprintf("error rendering Markdown: %s", err))
	}

	if v := ctx.Value(ContextKey{}); v != nil {
		container := v.(*ContextContainer)
		if _, ok := container.dependenciesMap[filename]; !ok {
			container.Dependencies = append(container.Dependencies, filename)
			container.dependenciesMap[filename] = struct{}{}
		}
	}

	s, err := mmarkdownext.Render(string(data), &mmarkdownext.RenderOptions{
		TemplateData: map[string]interface{}{
			"Ctx": ctx,
		},
	})
	if err != nil {
		panic(fmt.Sprintf("error rendering Markdown: %s", err))
	}

	return template.HTML(s)
}
