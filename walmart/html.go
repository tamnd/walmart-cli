package walmart

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// html.go holds the small shared helpers the surfaces lean on. Walmart renders
// its prices and item data inside a __NEXT_DATA__ JSON island (see nextdata.go),
// so most fields arrive already typed; these helpers clean the free text and
// read the few values that live in HTML attributes (category links, social-card
// meta tags).

var (
	priceRE = regexp.MustCompile(`([0-9][0-9,]*\.?[0-9]*)`)
	tagRE   = regexp.MustCompile(`<[^>]+>`)
)

// priceFromDisplay turns a displayed price ("$12.99", "Now $8.00") into a
// number, or 0 when there is none. Walmart prices are dollars, so the symbol is
// dropped and the first number wins.
func priceFromDisplay(s string) float64 {
	m := priceRE.FindString(s)
	if m == "" {
		return 0
	}
	v, err := strconv.ParseFloat(strings.ReplaceAll(m, ",", ""), 64)
	if err != nil {
		return 0
	}
	return v
}

// stripHTML reduces an HTML fragment to its text, collapsing whitespace. Walmart
// ships some descriptions as small HTML blobs.
func stripHTML(s string) string {
	return squish(tagRE.ReplaceAllString(s, " "))
}

// squish collapses runs of whitespace into single spaces and trims the ends, so
// a value lifted from indented markup reads cleanly.
func squish(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// firstText returns the trimmed text of the first selector that matches.
func firstText(s *goquery.Selection, selectors ...string) string {
	for _, sel := range selectors {
		if m := s.Find(sel).First(); m.Length() > 0 {
			if t := strings.TrimSpace(m.Text()); t != "" {
				return t
			}
		}
	}
	return ""
}

// metaContent returns the content of the first <meta> tag matching any key,
// trying both the property= and name= forms.
func metaContent(doc *goquery.Document, keys ...string) string {
	for _, k := range keys {
		for _, attr := range []string{"property", "name"} {
			if v, ok := doc.Find(`meta[` + attr + `="` + k + `"]`).First().Attr("content"); ok && v != "" {
				return v
			}
		}
	}
	return ""
}
