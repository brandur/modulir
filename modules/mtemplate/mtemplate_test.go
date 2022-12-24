package mtemplate

import (
	"context"
	"html/template"
	"net/url"
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
	fm1 := template.FuncMap{
		"CollapseParagraphs": CollapseParagraphs,
	}
	fm2 := template.FuncMap{
		"QueryEscape": QueryEscape,
	}
	fm3 := template.FuncMap{
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
	fm1 := template.FuncMap{
		"CollapseParagraphs": CollapseParagraphs,
	}
	fm2 := template.FuncMap{
		"CollapseParagraphs": CollapseParagraphs,
	}

	assert.PanicsWithError(t,
		"duplicate function map key on combine: CollapseParagraphs", func() {
			_ = CombineFuncMaps(fm1, fm2)
		})
}

func TestHTMLFuncMapToText(t *testing.T) {
	fm := template.FuncMap{
		"To2X": To2X,
	}

	textFM := HTMLFuncMapToText(fm)

	{
		_, ok := textFM["To2X"]
		assert.True(t, ok)
	}
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

func TestDownloadedImage(t *testing.T) {
	ctx := context.Background()

	t.Run("SetsContextAndEmitsPath", func(t *testing.T) {
		ctx, downloadedImageContainer := DownloadedImageContext(ctx)

		assert.Equal(t,
			"/photographs/belize/01/kukumba-beach-1.jpg",
			DownloadedImage(
				ctx,
				"/photographs/belize/01/kukumba-beach-1",
				"https://www.dropbox.com/s/6fmtgs00c5xtevg/2W4A1500.JPG?dl=1",
				1200,
			),
		)

		assert.Equal(t,
			[]*DownloadedImageInfo{
				{
					"/photographs/belize/01/kukumba-beach-1",
					mustURL(t, "https://www.dropbox.com/s/6fmtgs00c5xtevg/2W4A1500.JPG?dl=1"),
					1200,
				},
			},
			downloadedImageContainer.Images,
		)
	})

	t.Run("AlternateExtension", func(t *testing.T) {
		ctx, _ := DownloadedImageContext(ctx)

		assert.Equal(t,
			"/photographs/diagram.png",
			DownloadedImage(
				ctx,
				"/photographs/diagram",
				"https://www.dropbox.com/s/6fmtgs00c5xtevg/2W4A1500.png?dl=1",
				1200,
			),
		)
	})
}

func mustURL(t *testing.T, s string) *url.URL {
	t.Helper()
	u, err := url.Parse(s)
	assert.NoError(t, err)
	return u
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
	assert.Equal(t, "July 3, 2016 12:34", FormatTime(testTime, "January 2, 2006 15:04"))
}

func TestFormatTimeRFC3339UTC(t *testing.T) {
	assert.Equal(t, "2016-07-03T12:34:00Z", FormatTimeRFC3339UTC(testTime))
}

func TestFormatTimeSimpleDate(t *testing.T) {
	assert.Equal(t, "July 3, 2016", FormatTimeSimpleDate(testTime))
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

func TestMap(t *testing.T) {
	m := Map(MapVal("New", 456))
	assert.Contains(t, m, "New")
}

func TestMapValAdd(t *testing.T) {
	m := map[string]interface{}{
		"Preexisting": 123,
	}

	newM := MapValAdd(m, MapVal("New", 456))

	assert.Contains(t, newM, "Preexisting")
	assert.Contains(t, newM, "New")

	assert.Contains(t, m, "Preexisting")
	assert.NotContains(t, m, "New")
}

func TestQueryEscape(t *testing.T) {
	assert.Equal(t, "a%2Bb", QueryEscape("a+b"))
}

func TestRomanNumeral(t *testing.T) {
	assert.Equal(t, "I", RomanNumeral(1))
	assert.Equal(t, "II", RomanNumeral(2))
	assert.Equal(t, "III", RomanNumeral(3))
	assert.Equal(t, "IV", RomanNumeral(4))
	assert.Equal(t, "V", RomanNumeral(5))
	assert.Equal(t, "VI", RomanNumeral(6))
	assert.Equal(t, "VII", RomanNumeral(7))
	assert.Equal(t, "VIII", RomanNumeral(8))
	assert.Equal(t, "IX", RomanNumeral(9))
	assert.Equal(t, "X", RomanNumeral(10))
	assert.Equal(t, "XI", RomanNumeral(11))
	assert.Equal(t, "XII", RomanNumeral(12))
	assert.Equal(t, "XIII", RomanNumeral(13))
	assert.Equal(t, "XIV", RomanNumeral(14))
	assert.Equal(t, "XV", RomanNumeral(15))
	assert.Equal(t, "XVI", RomanNumeral(16))
	assert.Equal(t, "XVII", RomanNumeral(17))
	assert.Equal(t, "XVIII", RomanNumeral(18))
	assert.Equal(t, "XIX", RomanNumeral(19))
	assert.Equal(t, "XX", RomanNumeral(20))
	assert.Equal(t, "XXI", RomanNumeral(21))
	assert.Equal(t, "XL", RomanNumeral(40))
	assert.Equal(t, "L", RomanNumeral(50))
	assert.Equal(t, "LX", RomanNumeral(60))
	assert.Equal(t, "LXI", RomanNumeral(61))
	assert.Equal(t, "XC", RomanNumeral(90))
	assert.Equal(t, "C", RomanNumeral(100))
	assert.Equal(t, "CD", RomanNumeral(400))
	assert.Equal(t, "D", RomanNumeral(500))
	assert.Equal(t, "CM", RomanNumeral(900))
	assert.Equal(t, "M", RomanNumeral(1000))
	assert.Equal(t, "MCMXCIX", RomanNumeral(1999))
	assert.Equal(t, "MMMCMXCIX", RomanNumeral(3999))

	// Out of range
	assert.Equal(t, "0", RomanNumeral(0))
	assert.Equal(t, "4000", RomanNumeral(4000))
}

func TestRoundToString(t *testing.T) {
	assert.Equal(t, "1.2", RoundToString(1.234))
	assert.Equal(t, "1.0", RoundToString(1))
}

func TestTimeIn(t *testing.T) {
	tIn := TimeIn(testTime, "America/Los_Angeles")
	assert.Equal(t, "America/Los_Angeles", tIn.Location().String())
}

func TestTo2X(t *testing.T) {
	assert.Equal(t, template.HTML("/path/image@2x.jpg"), To2X("/path/image.jpg"))
	assert.Equal(t, template.HTML("/path/image@2x.png"), To2X("/path/image.png"))
	assert.Equal(t, template.HTML("image@2x.jpg"), To2X("image.jpg"))
	assert.Equal(t, template.HTML("image"), To2X("image"))
	assert.Equal(t, template.HTML("photos/reddit/rd_xxx_01/11%20-%20t9kxD78@2x.jpg"),
		To2X("photos/reddit/rd_xxx_01/11%20-%20t9kxD78.jpg"))
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
