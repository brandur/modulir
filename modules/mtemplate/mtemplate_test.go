package mtemplate

import (
	"html/template"
	"strings"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
)

var testTime time.Time

func init() {
	const longForm = "2006/01/02 15:04"
	var err error
	testTime, err = time.Parse(longForm, "2016/07/03 12:34")
	if err != nil {
		panic(err)
	}
}

func TestCollapseHTML(t *testing.T) {
	assert.Equal(t, "<p><strong>strong</strong></p>", collapseHTML(`
<p>
  <strong>strong</strong>
</p>`))
}

func TestCollapseParagraphs(t *testing.T) {
	assert.Equal(t, "<strong>strong</strong>", CollapseParagraphs(`
<p>
  <strong>strong</strong>
</p>
<p>
</p>`))
}

func TestCombineFuncMaps(t *testing.T) {
	var fm1 = template.FuncMap{
		"CollapseParagraphs": CollapseParagraphs,
	}
	var fm2 = template.FuncMap{
		"QueryEscape": QueryEscape,
	}
	var fm3 = template.FuncMap{
		"To2X": To2X,
	}

	combined := CombineFuncMaps(fm1, fm2, fm3)

	{
		_, ok := combined["CollapseParagraphs"]
		assert.True(t, ok)
	}
	{
		_, ok := combined["QueryEscape"]
		assert.True(t, ok)
	}
	{
		_, ok := combined["To2X"]
		assert.True(t, ok)
	}
}

func TestCombineFuncMaps_Duplicate(t *testing.T) {
	var fm1 = template.FuncMap{
		"CollapseParagraphs": CollapseParagraphs,
	}
	var fm2 = template.FuncMap{
		"CollapseParagraphs": CollapseParagraphs,
	}

	assert.PanicsWithError(t,
		"duplicate function map key on combine: CollapseParagraphs", func() {
			_ = CombineFuncMaps(fm1, fm2)
		})
}

func TestDistanceOfTimeInWords(t *testing.T) {
	to := time.Now()

	assert.Equal(t, "less than 1 minute",
		DistanceOfTimeInWords(to.Add(mustParseDuration("-1s")), to))
	assert.Equal(t, "1 minute",
		DistanceOfTimeInWords(to.Add(mustParseDuration("-1m")), to))
	assert.Equal(t, "8 minutes",
		DistanceOfTimeInWords(to.Add(mustParseDuration("-8m")), to))
	assert.Equal(t, "about 1 hour",
		DistanceOfTimeInWords(to.Add(mustParseDuration("-52m")), to))
	assert.Equal(t, "about 3 hours",
		DistanceOfTimeInWords(to.Add(mustParseDuration("-3h")), to))
	assert.Equal(t, "about 1 day",
		DistanceOfTimeInWords(to.Add(mustParseDuration("-24h")), to))

	// note that parse only handles up to "h" units
	assert.Equal(t, "9 days",
		DistanceOfTimeInWords(to.Add(mustParseDuration("-24h")*9), to))
	assert.Equal(t, "about 1 month",
		DistanceOfTimeInWords(to.Add(mustParseDuration("-24h")*30), to))
	assert.Equal(t, "4 months",
		DistanceOfTimeInWords(to.Add(mustParseDuration("-24h")*30*4), to))
	assert.Equal(t, "about 1 year",
		DistanceOfTimeInWords(to.Add(mustParseDuration("-24h")*365), to))
	assert.Equal(t, "about 1 year",
		DistanceOfTimeInWords(to.Add(mustParseDuration("-24h")*(365+2*30)), to))
	assert.Equal(t, "over 1 year",
		DistanceOfTimeInWords(to.Add(mustParseDuration("-24h")*(365+3*30)), to))
	assert.Equal(t, "almost 2 years",
		DistanceOfTimeInWords(to.Add(mustParseDuration("-24h")*(365+10*30)), to))
	assert.Equal(t, "2 years",
		DistanceOfTimeInWords(to.Add(mustParseDuration("-24h")*(365*2)), to))
	assert.Equal(t, "3 years",
		DistanceOfTimeInWords(to.Add(mustParseDuration("-24h")*(365*3)), to))
	assert.Equal(t, "10 years",
		DistanceOfTimeInWords(to.Add(mustParseDuration("-24h")*(365*10)), to))
}

func TestFigure(t *testing.T) {
	t.Run("SingleImage", func(t *testing.T) {
		assert.Equal(
			t,
			strings.TrimSpace(`
<figure>
    <img alt="alt" loading="lazy" src="src" srcset="src@2x 2x, src 1x">

    <figcaption>caption</figcaption>
</figure>
			`),
			string(Figure("caption", &HTMLImage{Src: "src", Alt: "alt"})),
		)
	})

	t.Run("MultipleImages", func(t *testing.T) {
		assert.Equal(
			t,
			strings.TrimSpace(`
<figure>
    <img alt="alt0" loading="lazy" src="src0" srcset="src0@2x 2x, src0 1x">
    <img alt="alt1" loading="lazy" src="src1" srcset="src1@2x 2x, src1 1x">
    <img alt="alt2" loading="lazy" src="src2" srcset="src2@2x 2x, src2 1x">

    <figcaption>caption</figcaption>
</figure>
			`),
			string(Figure(
				"caption",
				&HTMLImage{Src: "src0", Alt: "alt0"},
				&HTMLImage{Src: "src1", Alt: "alt1"},
				&HTMLImage{Src: "src2", Alt: "alt2"},
			)),
		)
	})
}

func TestFormatTime(t *testing.T) {
	assert.Equal(t, "July 3, 2016", FormatTime(&testTime))
}

func TestHTMLImageRender(t *testing.T) {
	t.Run("Basic", func(t *testing.T) {
		img := HTMLImage{Src: "src", Alt: "alt"}
		assert.Equal(
			t,
			`<img alt="alt" loading="lazy" src="src" srcset="src@2x 2x, src 1x">`,
			string(img.render()),
		)
	})

	t.Run("NoSrcsetForSVG", func(t *testing.T) {
		img := HTMLImage{Src: "src.svg", Alt: "alt"}
		assert.Equal(
			t,
			`<img alt="alt" loading="lazy" src="src.svg">`,
			string(img.render()),
		)
	})

	t.Run("WithClass", func(t *testing.T) {
		img := HTMLImage{Src: "src", Alt: "alt", Class: "class"}
		assert.Equal(
			t,
			`<img alt="alt" class="class" loading="lazy" src="src" srcset="src@2x 2x, src 1x">`,
			string(img.render()),
		)
	})
}

func TestHTMLRender(t *testing.T) {
	t.Run("SingleElement", func(t *testing.T) {
		assert.Equal(
			t,
			strings.TrimSpace(`
<img alt="alt" loading="lazy" src="src" srcset="src@2x 2x, src 1x">
			`),
			string(HTMLRender(
				&HTMLImage{Src: "src", Alt: "alt"},
			)),
		)
	})

	t.Run("MultipleElements", func(t *testing.T) {
		assert.Equal(
			t,
			strings.TrimSpace(`
<img alt="alt0" loading="lazy" src="src0" srcset="src0@2x 2x, src0 1x">
<img alt="alt1" loading="lazy" src="src1" srcset="src1@2x 2x, src1 1x">
<img alt="alt2" loading="lazy" src="src2" srcset="src2@2x 2x, src2 1x">
			`),
			string(HTMLRender(
				&HTMLImage{Src: "src0", Alt: "alt0"},
				&HTMLImage{Src: "src1", Alt: "alt1"},
				&HTMLImage{Src: "src2", Alt: "alt2"},
			)),
		)
	})
}

func TestHTMLSafePassThrough(t *testing.T) {
	assert.Equal(t, `{{print "x"}}`, string(HTMLSafePassThrough(`{{print "x"}}`)))
}

func TestImgSrcAndAlt(t *testing.T) {
	assert.Equal(t, HTMLImage{Src: "src", Alt: "alt"}, *ImgSrcAndAlt("src", "alt"))
}

func TestImgSrcAndAltAndClass(t *testing.T) {
	assert.Equal(
		t,
		HTMLImage{Src: "src", Alt: "alt", Class: "class"},
		*ImgSrcAndAltAndClass("src", "alt", "class"),
	)
}

func TestQueryEscape(t *testing.T) {
	assert.Equal(t, "a%2Bb", QueryEscape("a+b"))
}

func TestRoundToString(t *testing.T) {
	assert.Equal(t, "1.2", RoundToString(1.234))
	assert.Equal(t, "1.0", RoundToString(1))
}

func TestTo2X(t *testing.T) {
	assert.Equal(t, "/path/image@2x.jpg", To2X("/path/image.jpg"))
	assert.Equal(t, "/path/image@2x.png", To2X("/path/image.png"))
	assert.Equal(t, "image@2x.jpg", To2X("image.jpg"))
	assert.Equal(t, "image", To2X("image"))
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

func mustParseDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		panic(err)
	}
	return d
}
