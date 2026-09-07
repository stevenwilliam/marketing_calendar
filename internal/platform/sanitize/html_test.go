package sanitize

import "testing"

// The whole point of the allow-list: a rule that renders is not the same as a
// rule that can carry a payload.
func TestHTMLStripsEverythingDangerous(t *testing.T) {
	cases := []struct {
		name, in, mustNotContain string
	}{
		{"script", `<p>Diskon</p><script>alert(1)</script>`, "script"},
		{"event handler", `<p onclick="alert(1)">Diskon</p>`, "onclick"},
		{"img onerror", `<img src=x onerror=alert(1)>`, "onerror"},
		{"iframe", `<iframe src="//evil"></iframe>`, "iframe"},
		{"style attribute", `<p style="position:fixed">Diskon</p>`, "style"},
		{"class attribute", `<p class="x">Diskon</p>`, "class"},
		{"javascript link", `<a href="javascript:alert(1)">klik</a>`, "javascript"},
		{"svg", `<svg/onload=alert(1)>`, "onload"},
		{"data uri", `<a href="data:text/html,<script>">x</a>`, "data:"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := HTML(c.in+"<p>teks</p>", 5000)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if contains(got, c.mustNotContain) {
				t.Fatalf("%q survived sanitising: %q", c.mustNotContain, got)
			}
		})
	}
}

// The formatting a marketing person actually uses must survive, or they will
// paste from Word and fight the editor instead.
func TestHTMLKeepsTheFormattingWeAllow(t *testing.T) {
	in := `<p>Diskon <strong>20%</strong> untuk <em>gelas kedua</em>.</p>` +
		`<ul><li>Senin sampai Jumat</li><li>15.00–18.00</li></ul>`
	got, err := HTML(in, 5000)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"<strong>", "<em>", "<ul>", "<li>", "20%", "15.00–18.00"} {
		if !contains(got, want) {
			t.Fatalf("%q was removed: %q", want, got)
		}
	}
}

// The limit is on the TEXT, not the markup. Otherwise a user who has written
// three sentences is told they are over the limit because of their own bullets.
func TestLengthIsMeasuredOnTextNotMarkup(t *testing.T) {
	// 20 characters of text wrapped in far more than 20 characters of tags.
	in := `<ul><li><strong>satu dua tiga empat</strong></li></ul>`
	if _, err := HTML(in, 25); err != nil {
		t.Fatalf("19 characters of text must fit in a 25-character limit: %v", err)
	}
	long := "<p>" + repeat("a", 30) + "</p>"
	if _, err := HTML(long, 25); err != ErrTooLong {
		t.Fatalf("30 characters of text must exceed 25, got %v", err)
	}
}

// Markup that contains no words is empty, however much of it there is.
func TestMarkupWithoutTextIsEmpty(t *testing.T) {
	for _, in := range []string{"", "   ", "<p></p>", "<ul><li></li></ul>", "<script>alert(1)</script>"} {
		if _, err := HTML(in, 5000); err != ErrEmpty {
			t.Fatalf("%q should be empty, got %v", in, err)
		}
	}
}

// The CSV and the email get text, because markup in a spreadsheet cell is
// noise. A bulleted rule must not collapse into one run-on line.
func TestHTMLToTextIsReadable(t *testing.T) {
	got := HTMLToText(`<p>Diskon <strong>20%</strong>.</p><ul><li>Senin</li><li>Selasa</li></ul>`)
	want := "Diskon 20%.\n• Senin\n• Selasa"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if contains(got, "<") {
		t.Fatalf("markup survived into the text: %q", got)
	}
}

func contains(h, n string) bool {
	for i := 0; i+len(n) <= len(h); i++ {
		if h[i:i+len(n)] == n {
			return true
		}
	}
	return false
}

func repeat(s string, n int) string {
	out := ""
	for i := 0; i < n; i++ {
		out += s
	}
	return out
}
