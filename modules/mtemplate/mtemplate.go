package mtemplate

import (
	"fmt"
	"html/template"
	"math"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
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
	"FormatTime":                   FormatTime,
	"HTMLRender":                   HTMLRender,
	"HTMLSafePassThrough":          HTMLSafePassThrough,
	"ImgSrcAndAlt":                 ImgSrcAndAlt,
	"ImgSrcAndAltAndClass":         ImgSrcAndAltAndClass,
	"QueryEscape":                  QueryEscape,
	"RetinaImage":                  RetinaImage,
	"RetinaImageAlt":               RetinaImageAlt,
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
		out += fmt.Sprintf(`    %s
`,
			img.render())
	}

	out += fmt.Sprintf(`
    <figcaption>%s</figcaption>
</figure>`,
		figCaption)

	return template.HTML(strings.TrimSpace(out))
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

func (img *HTMLImage) render() template.HTML {
	// Giving everything for this type of image a lazy loading attribute seems
	// pretty safe given these are largely images that get embedded in blog
	// posts, etc.
	commonHTML := fmt.Sprintf(`<img src="%s" alt="%s" loading="lazy"`,
		img.Src, img.Alt)

	if img.Class != "" {
		return template.HTML(
			fmt.Sprintf(`%s class="%s">`, commonHTML, img.Class),
		)
	}

	return template.HTML(commonHTML + ">")
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

// RetinaImage produces an <img> tag containing a `srcset` with both the `@2x`
// and non-`@2x` version of the image.
func RetinaImage(source string) template.HTML {
	ext := filepath.Ext(source)
	retinaSource := strings.TrimSuffix(source, ext) + "@2x" + ext
	s := fmt.Sprintf(`<img src="%s" srcset="%s 2x, %s 1x" loading="lazy">`,
		source, retinaSource, source)
	return template.HTML(s)
}

// RetinaImageAlt produces an <img> tag containing a `srcset` with both the
// `@2x` and non-`@2x` version of the image. It also includes an alt.
func RetinaImageAlt(source, alt string) template.HTML {
	ext := filepath.Ext(source)
	retinaSource := strings.TrimSuffix(source, ext) + "@2x" + ext
	s := fmt.Sprintf(`<img src="%s" srcset="%s 2x, %s 1x" alt="%s" loading="lazy">`,
		source, retinaSource, source, strings.ReplaceAll(alt, `"`, `\"`))
	return template.HTML(s)
}

// RoundToString rounds a float to a presentation-friendly string.
func RoundToString(f float64) string {
	return fmt.Sprintf("%.1f", f)
}

// To2X takes a 1x (standad resolution) image path and changes it to a 2x path
// by putting `@2x` into its name right before its extension.
func To2X(imagePath string) string {
	parts := strings.Split(imagePath, ".")

	if len(parts) < 2 {
		return imagePath
	}

	parts[len(parts)-2] = parts[len(parts)-2] + "@2x"

	return strings.Join(parts, ".")
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
