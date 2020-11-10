package mmarkdownext

import (
	"testing"

	assert "github.com/stretchr/testify/require"
)

func TestCollapseHTML(t *testing.T) {
	assert.Equal(t, "<p><strong>strong</strong></p>", collapseHTML(`
  <p>
  <strong>strong</strong>
</p>`))
}

func TestRender(t *testing.T) {
	assert.Equal(t, "<p><strong>strong</strong></p>\n", must(Render("**strong**", nil)))
}

func TestTransformFootnotes(t *testing.T) {
	assert.Equal(t, `
<p>This is a reference <sup id="footnote-1-source"><a href="#footnote-1">1</a></sup> to a footnote <sup id="footnote-2-source"><a href="#footnote-2">2</a></sup>.</p>

<p>Not footnote: KEYS[1].</p>


<div class="footnotes">
  <p><sup id="footnote-1"><a href="#footnote-1-source">1</a></sup> Footnote one.</p>

<p><sup id="footnote-2"><a href="#footnote-2-source">2</a></sup> Footnote two.</p>

</div>
`,
		must(transformFootnotes(`
<p>This is a reference [1] to a footnote [2].</p>

<p>Not footnote: KEYS[1].</p>

<p>[1] Footnote one.</p>

<p>[2] Footnote two.</p>
`,
			nil,
		)),
	)

	// Without links
	assert.Equal(t, `
<p>This is a reference <sup><strong>1</strong></sup> to a footnote <sup><strong>2</strong></sup>.</p>

<p>Not footnote: KEYS[1].</p>


<div class="footnotes">
  <p><sup><strong>1</strong></sup> Footnote one.</p>

<p><sup><strong>2</strong></sup> Footnote two.</p>

</div>
`,
		must(transformFootnotes(`
<p>This is a reference [1] to a footnote [2].</p>

<p>Not footnote: KEYS[1].</p>

<p>[1] Footnote one.</p>

<p>[2] Footnote two.</p>
`,
			&RenderOptions{NoFootnoteLinks: true},
		)),
	)
}

func TestTransformHeaders(t *testing.T) {
	assert.Equal(t, `
<h2 id="intro" class="link"><a href="#intro">Introduction</a></h2>

Intro here.

<h2 id="section-1" class="link"><a href="#section-1">Body</a></h2>

<h3 id="article" class="link"><a href="#article">Article</a></h3>

Article one.

<h3 id="sub" class="link"><a href="#sub">Subsection</a></h3>

More content.

<h3 id="article-1" class="link"><a href="#article-1">Article</a></h3>

Article two.

<h3 id="section-5" class="link"><a href="#section-5">Subsection</a></h3>

More content.

<h2 id="conclusion" class="link"><a href="#conclusion">Conclusion</a></h2>

Conclusion.
`,
		must(transformHeaders(`
## Introduction (#intro)

Intro here.

## Body

### Article (#article)

Article one.

### Subsection (#sub)

More content.

### Article (#article)

Article two.

### Subsection

More content.

## Conclusion (#conclusion)

Conclusion.
`,
			nil,
		)),
	)

	assert.Equal(t, `
<h2>Introduction</h2>
`,
		must(transformHeaders(`
## Introduction (#intro)
`,
			&RenderOptions{NoHeaderLinks: true},
		)),
	)
}

func TestTransformImagesToRetina(t *testing.T) {
	assert.Equal(t,
		`<img src="/assets/hello.jpg" srcset="/assets/hello@2x.jpg 2x, /assets/hello.jpg 1x">`,
		must(transformImagesToRetina(`<img src="/assets/hello.jpg">`, nil)),
	)

	// No srcset is inserted for resolution agnostic SVGs.
	assert.Equal(t,
		`<img src="/assets/hello.svg">`,
		must(transformImagesToRetina(`<img src="/assets/hello.svg">`, nil)),
	)

	// Make sure transformation works with other attributes in the <img> tag (I
	// previously introduced a bug relating to this).
	assert.Equal(t,
		`<img src="/assets/hello.svg" class="overflowing">`,
		must(transformImagesToRetina(`<img src="/assets/hello.svg" class="overflowing">`, nil)),
	)

	// No replacement when we've explicitly requested no retina conversion
	assert.Equal(t,
		`<img src="/assets/hello.jpg">`,
		must(transformImagesToRetina(
			`<img src="/assets/hello.jpg">`,
			&RenderOptions{NoRetina: true},
		)),
	)
}

func must(v interface{}, err error) interface{} {
	if err != nil {
		panic(err)
	}
	return v
}
