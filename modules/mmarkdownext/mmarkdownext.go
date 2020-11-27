// Package mmarkdownext provides an extended version of Markdown that does
// several passes to add additional niceties like adding footnotes and allowing
// Go template helpers to be used..
package mmarkdownext

import (
	"bytes"
	"fmt"
	"text/template"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/brandur/modulir/modules/mtemplate"
	"github.com/pkg/errors"
	"gopkg.in/russross/blackfriday.v2"
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

// FuncMap is the map of helper functions that will be used when passing the
// Markdown through a Go template step.
var FuncMap = template.FuncMap{}

// RenderOptions describes a rendering operation to be customized.
type RenderOptions struct {
	// AbsoluteURL is the absolute URL of the final site. If set, the Markdown
	// renderer replaces the sources of any images or links that pointed to
	// relative URLs with absolute URLs.
	AbsoluteURL string

	// NoFollow adds `rel="nofollow"` to any external links.
	NoFollow bool

	// NoFootnoteLinks disables linking to and from footnotes.
	NoFootnoteLinks bool

	// NoHeaderLinks disables automatic permalinks on headers.
	NoHeaderLinks bool

	// NoRetina disables the Retina.JS rendering attributes.
	NoRetina bool
}

// Render a Markdown string to HTML while applying all custom project-specific
// filters including footnotes and stable header links.
func Render(s string, options *RenderOptions) (string, error) {
	var err error
	for _, f := range renderStack {
		s, err = f(s, options)
		if err != nil {
			return "", err
		}
	}
	return s, nil
}

//////////////////////////////////////////////////////////////////////////////
//
//
//
// Private
//
//
//
//////////////////////////////////////////////////////////////////////////////

// renderStack is the full set of functions that we'll run on an input string
// to get our fully rendered Markdown. This includes the rendering itself, but
// also a number of custom transformation options.
var renderStack = []func(string, *RenderOptions) (string, error){
	//
	// Pre-transformation functions
	//

	transformGoTemplate,
	transformHeaders,

	// DEPRECATED: Use Go template helpers instead.
	transformFigures,

	// The actual Blackfriday rendering
	func(source string, _ *RenderOptions) (string, error) {
		return string(blackfriday.Run([]byte(source))), nil
	},

	//
	// Post-transformation functions
	//

	// DEPRECATED: Find a different way to do this.
	transformCodeWithLanguagePrefix,

	transformFootnotes,

	// Should come before `transformImagesAndLinksToAbsoluteURLs` so that
	// relative links that are later converted to absolute aren't tagged with
	// `rel="nofollow"`.
	transformLinksToNoFollow,

	transformImagesAndLinksToAbsoluteURLs,
	transformImagesToRetina,
}

// Look for any whitespace between HTML tags.
var whitespaceRE = regexp.MustCompile(`>\s+<`)

// Simply collapses certain HTML snippets by removing newlines and whitespace
// between tags. This is mainline used to make HTML snippets readable as
// constants, but then to make them fit a little more nicely into the rendered
// markup.
func collapseHTML(html string) string {
	html = strings.Replace(html, "\n", "", -1)
	html = whitespaceRE.ReplaceAllString(html, "><")
	html = strings.TrimSpace(html)
	return html
}

var codeRE = regexp.MustCompile(`<code class="(\w+)">`)

func transformCodeWithLanguagePrefix(source string, options *RenderOptions) (string, error) {
	return codeRE.ReplaceAllString(source, `<code class="language-$1">`), nil
}

const figureHTML = `
<figure>
  <p><a href="%s"><img src="%s" class="overflowing"></a></p>
  <figcaption>%s</figcaption>
</figure>
`

var figureRE = regexp.MustCompile(`!fig src="(.*)" caption="(.*)"`)

func transformFigures(source string, options *RenderOptions) (string, error) {
	return figureRE.ReplaceAllStringFunc(source, func(figure string) string {
		matches := figureRE.FindStringSubmatch(figure)
		src := matches[1]

		link := src
		extension := filepath.Ext(link)
		if extension != "" && extension != ".svg" {
			link = link[0:len(src)-len(extension)] + "@2x" + extension
		}

		// This is a really ugly hack in that it relies on the regex above
		// being greedy about quotes, but meh, I'll make it better when there's
		// a good reason to.
		caption := strings.Replace(matches[2], `\"`, `"`, -1)

		return fmt.Sprintf(figureHTML, link, src, caption)
	}), nil
}

// Note that this should come early as we currently rely on a later step to
// give images a retina srcset.
func transformGoTemplate(source string, options *RenderOptions) (string, error) {
	// Skip this step if it doesn't look like there's any Go template code
	// contained in the source. (This may be a premature optimization.)
	if !strings.Contains(source, "{{") {
		return source, nil
	}

	tmpl, err := template.New("fmarkdownTemp").Funcs(FuncMap).Parse(source)
	if err != nil {
		return "", errors.Wrap(err, "error parsing template")
	}

	// Run the template to verify the output.
	var b bytes.Buffer
	err = tmpl.Execute(&b, nil)
	if err != nil {
		return "", errors.Wrap(err, "error executing template")
	}

	// fmt.Printf("output in = %v ...\n", b.String())
	return b.String(), nil
}

const headerHTML = `
<h%v id="%s" class="link">
	<a href="#%s">%s</a>
</h%v>
`

const headerHTMLNoLink = `
<h%v>%s</h%v>
`

// Matches one of the following:
//
//   # header
//   # header (#header-id)
//
// For now, only match ## or more so as to remove code comments from
// matches. We need a better way of doing that though.
var headerRE = regexp.MustCompile(`(?m:^(#{2,})\s+(.*?)(\s+\(#(.*)\))?$)`)

func transformHeaders(source string, options *RenderOptions) (string, error) {
	headerNum := 0

	// Tracks previously assigned headers so that we can detect duplicates.
	headers := make(map[string]int)

	source = headerRE.ReplaceAllStringFunc(source, func(header string) string {
		matches := headerRE.FindStringSubmatch(header)

		level := len(matches[1])
		title := matches[2]
		id := matches[4]

		var newID string

		if id == "" {
			// Header with no name, assign a prefixed number.
			newID = fmt.Sprintf("section-%v", headerNum)

		} else {
			occurrence, ok := headers[id]

			if ok {
				// Give duplicate IDs a suffix.
				newID = fmt.Sprintf("%s-%d", id, occurrence)
				headers[id]++

			} else {
				// Otherwise this is the first such ID we've seen.
				newID = id
				headers[id] = 1
			}
		}

		headerNum++

		// Replace the Markdown header with HTML equivalent.
		if options != nil && options.NoHeaderLinks {
			return collapseHTML(fmt.Sprintf(headerHTMLNoLink, level, title, level))
		}

		return collapseHTML(fmt.Sprintf(headerHTML, level, newID, newID, title, level))

	})

	return source, nil
}

// A layer that we wrap the entire footer section in for styling purposes.
const footerWrapper = `
<div class="footnotes">
  %s
</div>
`

// HTML for a footnote within the document.
const footnoteAnchorHTML = `
<sup id="footnote-%s">
  <a href="#footnote-%s-source">%s</a>
</sup>
`

// Same as footnoteAnchorHTML but without a link(this is used when sending
// emails).
const footnoteAnchorHTMLWithoutLink = `<sup><strong>%s</strong></sup>`

// HTML for a reference to a footnote within the document.
//
// Make sure there's a single space before the <sup> because we're replacing
// one as part of our search.
const footnoteReferenceHTML = `
<sup id="footnote-%s-source">
  <a href="#footnote-%s">%s</a>
</sup>
`

// Same as footnoteReferenceHTML but without a link (this is used when sending
// emails).
//
// Make sure there's a single space before the <sup> because we're replacing
// one as part of our search.
const footnoteReferenceHTMLWithoutLink = `<sup><strong>%s</strong></sup>`

// Look for the section the section at the bottom of the page that looks like
// <p>[1] (the paragraph tag is there because Markdown will have already
// wrapped it by this point).
var footerRE = regexp.MustCompile(`(?ms:^<p>\[\d+\].*)`)

// Look for a single footnote within the footer.
var footnoteRE = regexp.MustCompile(`\[(\d+)\](\s+.*)`)

// Note that this must be a post-transform filter. If it wasn't, our Markdown
// renderer would not render the Markdown inside the footnotes layer because it
// would already be wrapped in HTML.
func transformFootnotes(source string, options *RenderOptions) (string, error) {
	footer := footerRE.FindString(source)

	if footer != "" {
		// remove the footer for now
		source = strings.Replace(source, footer, "", 1)

		footer = footnoteRE.ReplaceAllStringFunc(footer, func(footnote string) string {
			// first create a footnote with an anchor that links can target
			matches := footnoteRE.FindStringSubmatch(footnote)
			number := matches[1]

			var anchor string
			if options != nil && options.NoFootnoteLinks {
				anchor = fmt.Sprintf(footnoteAnchorHTMLWithoutLink, number) + matches[2]
			} else {
				anchor = fmt.Sprintf(footnoteAnchorHTML, number, number, number) + matches[2]
			}

			// Then replace all references in the body to this footnote.
			//
			// Note the leading space before ` [%s]`. This is a little hacky,
			// but is there to try and ensure that we don't try to replace
			// strings that look like footnote references, but aren't.
			// `KEYS[1]` from `/redis-cluster` is an example of one of these
			// strings that might be a false positive.
			var reference string
			if options != nil && options.NoFootnoteLinks {
				reference = fmt.Sprintf(footnoteReferenceHTMLWithoutLink, number)
			} else {
				reference = fmt.Sprintf(footnoteReferenceHTML, number, number, number)
			}
			source = strings.Replace(source,
				fmt.Sprintf(` [%s]`, number),
				" "+collapseHTML(reference), -1)

			return collapseHTML(anchor)
		})

		// and wrap the whole footer section in a layer for styling
		footer = fmt.Sprintf(footerWrapper, footer)
		source = source + footer
	}

	return source, nil
}

var imageRE = regexp.MustCompile(`<img src="([^"]+)"([^>]*)`)

func transformImagesToRetina(source string, options *RenderOptions) (string, error) {
	if options != nil && options.NoRetina {
		return source, nil
	}

	// The basic idea here is that we give every image a `srcset` that includes
	// 2x so that browsers will replace it with a retina version.
	return imageRE.ReplaceAllStringFunc(source, func(img string) string {
		matches := imageRE.FindStringSubmatch(img)

		// SVGs are resolution-agnostic and don't need replacing.
		if filepath.Ext(matches[1]) == ".svg" {
			return fmt.Sprintf(`<img src="%s"%s`, matches[1], matches[2])
		}

		// If the image already has a srcset, do nothing.
		if strings.Contains(matches[2], "srcset") {
			return fmt.Sprintf(`<img src="%s"%s`, matches[1], matches[2])
		}

		return fmt.Sprintf(`<img src="%s" srcset="%s 2x, %s 1x"%s`,
			matches[1],
			mtemplate.To2X(matches[1]),
			matches[1],
			matches[2],
		)
	}), nil
}

var relativeImageRE = regexp.MustCompile(`<img src="/`)

var relativeLinkRE = regexp.MustCompile(`<a href="/`)

func transformImagesAndLinksToAbsoluteURLs(source string, options *RenderOptions) (string, error) {
	if options == nil || options.AbsoluteURL == "" {
		return source, nil
	}

	source = relativeImageRE.ReplaceAllStringFunc(source, func(img string) string {
		return `<img src="` + options.AbsoluteURL + `/`
	})

	source = relativeLinkRE.ReplaceAllStringFunc(source, func(img string) string {
		return `<a href="` + options.AbsoluteURL + `/`
	})

	return source, nil
}

var absoluteLinkRE = regexp.MustCompile(`<a href="http[^"]+"`)

func transformLinksToNoFollow(source string, options *RenderOptions) (string, error) {
	if options == nil || !options.NoFollow {
		return source, nil
	}

	return absoluteLinkRE.ReplaceAllStringFunc(source, func(link string) string {
		return fmt.Sprintf(`%s rel="nofollow"`, link)
	}), nil
}
