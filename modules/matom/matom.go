package matom

import (
	"encoding/xml"
	"io"
	"time"

	"golang.org/x/xerrors"
)

// Category is a category of an Atom entry.
type Category struct {
	XMLName struct{} `xml:"category"`

	Term string `xml:"term,attr"`
}

// Entry is a single entry in an Atom feed.
type Entry struct {
	XMLName struct{} `xml:"entry"`

	Title     string        `xml:"title"`
	Summary   string        `xml:"summary,omitempty"`
	Content   *EntryContent `xml:"content"`
	Published time.Time     `xml:"published"`
	Updated   time.Time     `xml:"updated"`
	Link      *Link         `xml:""`
	ID        string        `xml:"id"`

	AuthorName string `xml:"author>name,omitempty"`
	AuthorURI  string `xml:"author>uri,omitempty"`

	Categories []*Category `xml:""`
}

// EntryContent is a simple helper class that allows us to wrap an entry's
// content in an XML CDATA tag.
type EntryContent struct {
	Content string `xml:",cdata"`
	Type    string `xml:"type,attr,omitempty"`
}

// Feed represents an Atom feed that with be marshaled to XML.
//
// Note that XMLName is a Golang XML "magic" attribute.
type Feed struct {
	XMLName struct{} `xml:"feed"`

	XMLLang string `xml:"xml:lang,attr"`
	XMLNS   string `xml:"xmlns,attr"`

	Title   string    `xml:"title"`
	ID      string    `xml:"id"`
	Updated time.Time `xml:"updated"`

	Links   []*Link  `xml:""`
	Entries []*Entry `xml:""`
}

// Link is a link embedded in the header of an Atom feed.
type Link struct {
	XMLName struct{} `xml:"link"`

	Rel  string `xml:"rel,attr,omitempty"`
	Type string `xml:"type,attr,omitempty"`
	Href string `xml:"href,attr"`
}

// Encode the feed to an io.Writer.
//
// Adds a few attributes that have mostly default content like xml:lang and
// xmlns.
func (f *Feed) Encode(w io.Writer, indent string) error {
	if f.XMLLang == "" {
		f.XMLLang = "en-US"
	}

	if f.XMLNS == "" {
		f.XMLNS = "http://www.w3.org/2005/Atom"
	}

	_, err := w.Write([]byte(xml.Header))
	if err != nil {
		return xerrors.Errorf("error writing Atom feed header: %w", err)
	}

	enc := xml.NewEncoder(w)
	enc.Indent("", indent)
	if err := enc.Encode(f); err != nil {
		return xerrors.Errorf("error encoding Atom feed: %w", err)
	}

	return nil
}
