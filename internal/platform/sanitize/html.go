package sanitize

import (
	"strings"
	"unicode/utf8"

	"github.com/microcosm-cc/bluemonday"
	"golang.org/x/net/html"
)

// richText is the allow-list for the one rich-text field in the product, the
// promotion rule.
//
// It is deliberately tiny. Every tag on this list is one a marketing person
// actually reaches for when writing "20% off the second cup, 15:00–18:00";
// everything else — images, tables, iframes, styles, classes, ids, links — is
// removed. A rule that renders is not the same as a rule that can carry a
// payload, and the list is the difference.
//
// No `href` on purpose: a link in an internal promotion rule buys nothing and
// brings the whole `javascript:` and open-redirect surface with it.
var richText = func() *bluemonday.Policy {
	p := bluemonday.NewPolicy()
	p.AllowElements("p", "br", "strong", "b", "em", "i", "u", "s",
		"ul", "ol", "li", "h3", "h4", "blockquote")
	// Nothing may carry an attribute. No style, no class, no data-*.
	p.RequireNoFollowOnLinks(false)
	return p
}()

// HTML sanitises rich text on the way IN, so what is stored is already safe
// and every later reader — the detail screen, an export, a notification —
// inherits that rather than each having to remember.
//
// It returns the cleaned HTML and its plain-text length. The LENGTH IS
// MEASURED ON THE TEXT, not the markup: otherwise a limit of 5,000 is spent
// on tags, and a user who has written three sentences is told they are over.
func HTML(s string, maxTextRunes int) (string, error) {
	if !utf8.ValidString(s) {
		return "", ErrNotUTF8
	}
	clean := strings.TrimSpace(richText.Sanitize(s))
	// Emptiness and length are measured on the WORDS ONLY — not on
	// HTMLToText's output, which adds a bullet for every <li>. An empty
	// bulleted list flattens to "•", which is not empty, and a required field
	// would have accepted a visually blank rule.
	text := strings.TrimSpace(textContent(clean))
	if text == "" {
		return "", ErrEmpty
	}
	if utf8.RuneCountInString(text) > maxTextRunes {
		return "", ErrTooLong
	}
	return clean, nil
}

// textContent is every text node, concatenated, with no decoration added.
// It answers "are there any words here" and "how many" — nothing else.
func textContent(s string) string {
	doc, err := html.Parse(strings.NewReader(s))
	if err != nil {
		return stripTagsCrudely(s)
	}
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return strings.TrimSpace(b.String())
}

// HTMLToText flattens markup to readable plain text.
//
// Used for the CSV export and for email, where markup is noise rather than
// formatting. Block elements become newlines so a bulleted rule does not
// collapse into one run-on line.
func HTMLToText(s string) string {
	doc, err := html.Parse(strings.NewReader(s))
	if err != nil {
		// Parsing cannot really fail for a fragment, but a fallback that
		// returns the raw markup would put tags in a spreadsheet cell.
		return stripTagsCrudely(s)
	}
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		if n.Type == html.ElementNode {
			switch n.Data {
			case "li":
				b.WriteString("• ")
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
		if n.Type == html.ElementNode {
			switch n.Data {
			case "p", "br", "li", "h3", "h4", "blockquote", "ul", "ol":
				b.WriteString("\n")
			}
		}
	}
	walk(doc)

	// Collapse the runs of blank lines the block rule above produces.
	lines := strings.Split(b.String(), "\n")
	var out []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" && len(out) > 0 && out[len(out)-1] == "" {
			continue
		}
		out = append(out, l)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

func stripTagsCrudely(s string) string {
	var b strings.Builder
	depth := 0
	for _, r := range s {
		switch r {
		case '<':
			depth++
		case '>':
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 {
				b.WriteRune(r)
			}
		}
	}
	return strings.TrimSpace(b.String())
}
