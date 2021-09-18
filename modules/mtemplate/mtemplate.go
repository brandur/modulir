package mtemplate

import (
	"fmt"
	"html/template"
	"math"
	"net/url"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	texttemplate "text/template"
	"time"
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

// FuncMap is a set of helper functions to make available in templates for the
// project.
var FuncMap = template.FuncMap{
	"CollapseParagraphs":           CollapseParagraphs,
	"DistanceOfTimeInWords":        DistanceOfTimeInWords,
	"DistanceOfTimeInWordsFromNow": DistanceOfTimeInWordsFromNow,
	"Figure":                       Figure,
	"FigureSingle":                 FigureSingle,
	"FigureSingleWithClass":        FigureSingleWithClass,
	"FormatTime":                   FormatTime,
	"HTMLRender":                   HTMLRender,
	"HTMLSafePassThrough":          HTMLSafePassThrough,
	"ImgSrcAndAlt":                 ImgSrcAndAlt,
	"ImgSrcAndAltAndClass":         ImgSrcAndAltAndClass,
	"QueryEscape":                  QueryEscape,
	"RoundToString":                RoundToString,
	"To2X":                         To2X,
}

// CollapseParagraphs strips paragraph tags out of rendered HTML. Note that it
// does not handle HTML with any attributes, so is targeted mainly for use with
// HTML generated from Markdown.
func CollapseParagraphs(s string) string {
	sCollapsed := s
	sCollapsed = strings.Replace(sCollapsed, "<p>", "", -1)
	sCollapsed = strings.Replace(sCollapsed, "</p>", "", -1)
	return collapseHTML(sCollapsed)
}

// CombineFuncMaps combines a number of function maps into one. The combined
// version is a new function map so that none of the originals are tainted.
func CombineFuncMaps(funcMaps ...template.FuncMap) template.FuncMap {
	// Combine both sets of helpers into a single untainted function map.
	combined := make(template.FuncMap)

	for _, fm := range funcMaps {
		for k, v := range fm {
			if _, ok := combined[k]; ok {
				panic(fmt.Errorf("duplicate function map key on combine: %s", k))
			}

			combined[k] = v
		}
	}

	return combined
}

// HTMLFuncMapToText transforms an HTML func map to a text func map.
func HTMLFuncMapToText(funcMap template.FuncMap) texttemplate.FuncMap {
	textFuncMap := make(texttemplate.FuncMap)

	for k, v := range funcMap {
		textFuncMap[k] = v
	}

	return textFuncMap
}

const (
	minutesInDay   = 24 * 60
	minutesInMonth = 30 * 24 * 60
	minutesInYear  = 365 * 24 * 60
)

// DistanceOfTimeInWords returns a string describing the relative time passed
// between two times.
func DistanceOfTimeInWords(to, from time.Time) string {
	d := from.Sub(to)
	min := int(round(d.Minutes()))

	if min == 0 {
		return "less than 1 minute"
	} else if min == 1 {
		return fmt.Sprintf("%d minute", min)
	} else if min >= 1 && min <= 44 {
		return fmt.Sprintf("%d minutes", min)
	} else if min >= 45 && min <= 89 {
		return "about 1 hour"
	} else if min >= 90 && min <= minutesInDay-1 {
		return fmt.Sprintf("about %d hours", int(round(d.Hours())))
	} else if min >= minutesInDay && min <= minutesInDay*2-1 {
		return "about 1 day"
	} else if min >= 2520 && min <= minutesInMonth-1 {
		return fmt.Sprintf("%d days", int(round(d.Hours()/24.0)))
	} else if min >= minutesInMonth && min <= minutesInMonth*2-1 {
		return "about 1 month"
	} else if min >= minutesInMonth*2 && min <= minutesInYear-1 {
		return fmt.Sprintf("%d months", int(round(d.Hours()/24.0/30.0)))
	} else if min >= minutesInYear && min <= minutesInYear+3*minutesInMonth-1 {
		return "about 1 year"
	} else if min >= minutesInYear+3*minutesInMonth-1 && min <= minutesInYear+9*minutesInMonth-1 {
		return "over 1 year"
	} else if min >= minutesInYear+9*minutesInMonth && min <= minutesInYear*2-1 {
		return "almost 2 years"
	}

	return fmt.Sprintf("%d years", int(round(d.Hours()/24.0/365.0)))
}

// DistanceOfTimeInWordsFromNow returns a string describing the relative time
// passed between a time and the current moment.
func DistanceOfTimeInWordsFromNow(to time.Time) string {
	return DistanceOfTimeInWords(to, time.Now())
}

// Figure wraps a number of images into a figure and assigns them a caption as
// well as alt text.
func Figure(figCaption string, imgs ...*HTMLImage) template.HTML {
	out := `
<figure>
`

	for _, img := range imgs {
		out += "    " + string(img.render()) + "\n"
	}

	out += fmt.Sprintf(`    <figcaption>%s</figcaption>
</figure>`,
		figCaption)

	return template.HTML(strings.TrimSpace(out))
}

// FigureSingle is a shortcut for creating a simple figure with a single image
// and with an alt that matches the caption.
func FigureSingle(figCaption, src string) template.HTML {
	return Figure(figCaption, &HTMLImage{Alt: figCaption, Src: src})
}

// FigureSingleWithClass is a shortcut for creating a simple figure with a
// single image and with an alt that matches the caption, and with an HTML
// class..
func FigureSingleWithClass(figCaption, src, class string) template.HTML {
	return Figure(figCaption, &HTMLImage{Alt: figCaption, Class: class, Src: src})
}

// HTMLSafePassThrough passes a string through to the final render. This is
// especially useful for code samples that contain Go template syntax which
// shouldn't be rendered.
func HTMLSafePassThrough(s string) template.HTML {
	return template.HTML(strings.TrimSpace(s))
}

// HTMLElement represents an HTML element that can be rendered.
type HTMLElement interface {
	render() template.HTML
}

// HTMLImage is a simple struct representing an HTML image to be rendered and
// some of the attributes it might have.
type HTMLImage struct {
	Src   string
	Alt   string
	Class string
}

// htmlElementRenderer is an internal representation of an HTML element to make
// building one with a set of properties easier.
type htmlElementRenderer struct {
	Name  string
	Attrs map[string]string
}

func (r *htmlElementRenderer) render() template.HTML {
	var pairs []string
	for name, val := range r.Attrs {
		pairs = append(pairs, fmt.Sprintf(`%s="%s"`, name, val))
	}

	// Sort the outgoing names so that we have something stable to test against
	sort.Strings(pairs)

	return template.HTML(fmt.Sprintf(
		`<%s %s>`,
		r.Name,
		strings.Join(pairs, " "),
	))
}

func (img *HTMLImage) render() template.HTML {
	element := htmlElementRenderer{
		Name: "img",
		Attrs: map[string]string{
			"alt":     img.Alt,
			"loading": "lazy",
			"src":     img.Src,
		},
	}

	ext := filepath.Ext(img.Src)
	if ext != ".svg" {
		retinaSource := strings.TrimSuffix(img.Src, ext) + "@2x" + ext
		element.Attrs["srcset"] = fmt.Sprintf("%s 2x, %s 1x", retinaSource, img.Src)
	}

	if img.Class != "" {
		element.Attrs["class"] = img.Class
	}

	return element.render()
}

// HTMLRender renders a series of mtemplate HTML elements.
func HTMLRender(elements ...HTMLElement) template.HTML {
	rendered := make([]string, len(elements))

	for i, element := range elements {
		rendered[i] = string(element.render())
	}

	return template.HTML(
		strings.Join(rendered, "\n"),
	)
}

// ImgSrcAndAlt is a shortcut for creating ImgSrcAndAlt.
func ImgSrcAndAlt(imgSrc, imgAlt string) *HTMLImage {
	return &HTMLImage{imgSrc, imgAlt, ""}
}

// ImgSrcAndAltAndClass is a shortcut for creating ImgSrcAndAlt with a CSS
// class.
func ImgSrcAndAltAndClass(imgSrc, imgAlt, class string) *HTMLImage {
	return &HTMLImage{imgSrc, imgAlt, class}
}

// FormatTime formats time according to a relatively straightforward time
// format.
func FormatTime(t *time.Time) string {
	return toNonBreakingWhitespace(t.Format("January 2, 2006"))
}

// QueryEscape escapes a URL.
func QueryEscape(s string) string {
	return url.QueryEscape(s)
}

// RoundToString rounds a float to a presentation-friendly string.
func RoundToString(f float64) string {
	return fmt.Sprintf("%.1f", f)
}

// To2X takes a 1x (standad resolution) image path and changes it to a 2x path
// by putting `@2x` into its name right before its extension.
func To2X(imagePath string) template.HTML {
	parts := strings.Split(imagePath, ".")

	if len(parts) < 2 {
		return template.HTML(imagePath)
	}

	parts[len(parts)-2] = parts[len(parts)-2] + "@2x"

	return template.HTML(strings.Join(parts, "."))
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

// There is no "round" function built into Go :/
func round(f float64) float64 {
	return math.Floor(f + .5)
}

func toNonBreakingWhitespace(str string) string {
	return strings.Replace(str, " ", " ", -1)
}
