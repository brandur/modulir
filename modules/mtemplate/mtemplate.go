package mtemplate

import (
	"context"
	"fmt"
	"html/template"
	"math"
	"net/url"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	texttemplate "text/template"
	"time"

	"golang.org/x/xerrors"
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
	"DownloadedImage":              DownloadedImage,
	"Figure":                       Figure,
	"FigureSingle":                 FigureSingle,
	"FigureSingleWithClass":        FigureSingleWithClass,
	"FormatTime":                   FormatTime,
	"FormatTimeRFC3339UTC":         FormatTimeRFC3339UTC,
	"FormatTimeSimpleDate":         FormatTimeSimpleDate,
	"HTMLRender":                   HTMLRender,
	"HTMLSafePassThrough":          HTMLSafePassThrough,
	"ImgSrcAndAlt":                 ImgSrcAndAlt,
	"ImgSrcAndAltAndClass":         ImgSrcAndAltAndClass,
	"Map":                          Map,
	"MapVal":                       MapVal,
	"MapValAdd":                    MapValAdd,
	"QueryEscape":                  QueryEscape,
	"RomanNumeral":                 RomanNumeral,
	"RoundToString":                RoundToString,
	"TimeIn":                       TimeIn,
	"To2X":                         To2X,
}

// CollapseParagraphs strips paragraph tags out of rendered HTML. Note that it
// does not handle HTML with any attributes, so is targeted mainly for use with
// HTML generated from Markdown.
func CollapseParagraphs(s string) string {
	sCollapsed := s
	sCollapsed = strings.ReplaceAll(sCollapsed, "<p>", "")
	sCollapsed = strings.ReplaceAll(sCollapsed, "</p>", "")
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
				panic(xerrors.Errorf("duplicate function map key on combine: %s", k))
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

	switch {
	case min == 0:
		return "less than 1 minute"
	case min == 1:
		return fmt.Sprintf("%d minute", min)
	case min >= 1 && min <= 44:
		return fmt.Sprintf("%d minutes", min)
	case min >= 45 && min <= 89:
		return "about 1 hour"
	case min >= 90 && min <= minutesInDay-1:
		return fmt.Sprintf("about %d hours", int(round(d.Hours())))
	case min >= minutesInDay && min <= minutesInDay*2-1:
		return "about 1 day"
	case min >= 2520 && min <= minutesInMonth-1:
		return fmt.Sprintf("%d days", int(round(d.Hours()/24.0)))
	case min >= minutesInMonth && min <= minutesInMonth*2-1:
		return "about 1 month"
	case min >= minutesInMonth*2 && min <= minutesInYear-1:
		return fmt.Sprintf("%d months", int(round(d.Hours()/24.0/30.0)))
	case min >= minutesInYear && min <= minutesInYear+3*minutesInMonth-1:
		return "about 1 year"
	case min >= minutesInYear+3*minutesInMonth-1 && min <= minutesInYear+9*minutesInMonth-1:
		return "over 1 year"
	case min >= minutesInYear+9*minutesInMonth && min <= minutesInYear*2-1:
		return "almost 2 years"
	}

	return fmt.Sprintf("%d years", int(round(d.Hours()/24.0/365.0)))
}

// DistanceOfTimeInWordsFromNow returns a string describing the relative time
// passed between a time and the current moment.
func DistanceOfTimeInWordsFromNow(to time.Time) string {
	return DistanceOfTimeInWords(to, time.Now())
}

type downloadedImageContextKey struct{}

type DownloadedImageContextContainer struct {
	Images []*DownloadedImageInfo
}

type DownloadedImageInfo struct {
	Slug  string
	URL   *url.URL
	Width int

	// Internal
	ext string `toml:"-"`
}

func (p *DownloadedImageInfo) OriginalExt() string {
	if p.ext != "" {
		return p.ext
	}

	p.ext = strings.ToLower(filepath.Ext(p.URL.Path))
	return p.ext
}

func DownloadedImageContext(ctx context.Context) (context.Context, *DownloadedImageContextContainer) {
	container := &DownloadedImageContextContainer{}
	return context.WithValue(ctx, downloadedImageContextKey{}, container), container
}

// DownloadedImage represents an image that's available remotely, and which will
// be downloaded and stored as the local target slug. This doesn't happen
// automatically though -- DownloadedImageContext must be called first to set a
// context container, and from there any downloaded image slugs and URLs can be
// extracted after all sources are rendered to be sent to mimage for processing.
func DownloadedImage(ctx context.Context, slug, imageURL string, width int) string {
	v := ctx.Value(downloadedImageContextKey{})
	if v == nil {
		panic("context key not set; DownloadedImageContext must be called")
	}

	u, err := url.Parse(imageURL)
	if err != nil {
		panic(fmt.Sprintf("error parsing image URL %q: %v", imageURL, err))
	}

	container := v.(*DownloadedImageContextContainer)
	container.Images = append(container.Images, &DownloadedImageInfo{slug, u, width, ""})

	return slug + strings.ToLower(filepath.Ext(u.Path))
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

	if figCaption != "" {
		out += fmt.Sprintf(`    <figcaption>%s</figcaption>`+"\n", figCaption)
	}

	out += "</figure>"

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
	pairs := make([]string, 0, len(r.Attrs))
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
			"loading": "lazy",
			"src":     img.Src,
		},
	}

	if img.Alt != "" {
		element.Attrs["alt"] = img.Alt
	}

	if ext := filepath.Ext(img.Src); ext != ".svg" {
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

// FormatTime formats time according to the given format string.
func FormatTime(t time.Time, format string) string {
	return toNonBreakingWhitespace(t.Format(format))
}

// FormatTime formats time according to the given format string.
func FormatTimeRFC3339UTC(t time.Time) string {
	return toNonBreakingWhitespace(t.UTC().Format(time.RFC3339))
}

// FormatTimeSimpleDate formats time according to a relatively straightforward
// time format.
func FormatTimeSimpleDate(t time.Time) string {
	return toNonBreakingWhitespace(t.Format("January 2, 2006"))
}

type mapVal struct {
	key string
	val interface{}
}

func Map(vals ...*mapVal) map[string]interface{} {
	m := make(map[string]interface{})

	for _, val := range vals {
		m[val.key] = val.val
	}

	return m
}

// MapVal generates a new map key/value for use with MapValAdd.
func MapVal(key string, val interface{}) *mapVal { //nolint:revive
	return &mapVal{key, val}
}

// MapValAdd is a convenience helper for adding a new key and value to a shallow
// copy of the given map and returning it.
func MapValAdd(m map[string]interface{}, vals ...*mapVal) map[string]interface{} {
	mCopy := make(map[string]interface{}, len(m))

	for k, v := range m {
		mCopy[k] = v
	}

	for _, val := range vals {
		mCopy[val.key] = val.val
	}

	return mCopy
}

// QueryEscape escapes a URL.
func QueryEscape(s string) string {
	return url.QueryEscape(s)
}

func RomanNumeral(num int) string {
	const maxRomanNumber int = 3999

	if num > maxRomanNumber || num < 1 {
		return strconv.Itoa(num)
	}

	conversions := []struct {
		value int
		digit string
	}{
		{1000, "M"},
		{900, "CM"},
		{500, "D"},
		{400, "CD"},
		{100, "C"},
		{90, "XC"},
		{50, "L"},
		{40, "XL"},
		{10, "X"},
		{9, "IX"},
		{5, "V"},
		{4, "IV"},
		{1, "I"},
	}

	var roman strings.Builder
	for _, conversion := range conversions {
		for num >= conversion.value {
			roman.WriteString(conversion.digit)
			num -= conversion.value
		}
	}

	return roman.String()
}

// RoundToString rounds a float to a presentation-friendly string.
func RoundToString(f float64) string {
	return fmt.Sprintf("%.1f", f)
}

func TimeIn(t time.Time, locationName string) time.Time {
	location, err := time.LoadLocation(locationName)
	if err != nil {
		panic(err)
	}
	return t.In(location)
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
	html = strings.ReplaceAll(html, "\n", "")
	html = whitespaceRE.ReplaceAllString(html, "><")
	html = strings.TrimSpace(html)
	return html
}

// There is no "round" function built into Go :/.
func round(f float64) float64 {
	return math.Floor(f + .5)
}

func toNonBreakingWhitespace(str string) string {
	return strings.ReplaceAll(str, " ", " ")
}
