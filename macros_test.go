package frontmatter

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

var macroMeta = map[string]any{
	"title":    "Hello",
	"author":   "Dave",
	"pub_date": "Fri, 23 Oct 2026 15:19:45 GMT",
	"source":   map[string]any{"title": "Scripting News", "url": "http://scripting.com/"},
	"tags":     []any{"a", "b"},
	"count":    3,
}

// --- Expand: Markdown in, Markdown out (ported from cmarkbrowser render.Expand) ---

func TestExpandPageMacros(t *testing.T) {
	src := "# {{page.title}}\n\nBy {{page.author}} on {{page.pub_date}}. {{page.count}} items.\n"
	got, rep := Expand(src, macroMeta)
	want := "# Hello\n\nBy Dave on Fri, 23 Oct 2026 15:19:45 GMT. 3 items.\n"
	if got != want || len(rep.Missing) != 0 || len(rep.Warnings) != 0 {
		t.Errorf("got %q report %+v, want %q", got, rep, want)
	}
}

func TestExpandNestedAndListKeys(t *testing.T) {
	got, rep := Expand("From {{page.source.title}} ({{page.tags}})\n", macroMeta)
	if got != "From Scripting News (a, b)\n" || len(rep.Missing) != 0 {
		t.Errorf("got %q report %+v", got, rep)
	}
}

func TestExpandLeavesCodeAlone(t *testing.T) {
	src := "Use `{{page.title}}` here.\n\n```\n{{page.title}}\n```\n\n    {{page.title}}\n\n{{page.title}}\n"
	got, _ := Expand(src, macroMeta)
	if strings.Count(got, "{{page.title}}") != 3 || strings.Count(got, "Hello") != 1 {
		t.Errorf("code was expanded or text was not: %q", got)
	}
}

func TestExpandIsOnePass(t *testing.T) {
	meta := map[string]any{"title": "{{page.author}}", "author": "Dave"}
	got, _ := Expand("{{page.title}}\n", meta)
	if strings.Contains(got, "Dave") || !strings.Contains(got, "page.author") {
		t.Errorf("a value was expanded again: %q", got)
	}
}

func TestExpandMissingKeyStaysVisibleAndIsReportedOnce(t *testing.T) {
	got, rep := Expand("{{page.nope}} {{page.nope}} and {{page.title}} and {{page.source}}\n", macroMeta)
	if !strings.Contains(got, "{{page.nope}} {{page.nope}}") || !strings.Contains(got, "Hello") {
		t.Errorf("got %q", got)
	}
	if !strings.Contains(got, "{{page.source}}") { // a table is as good as missing
		t.Errorf("table key was expanded: %q", got)
	}
	if len(rep.Missing) != 2 || rep.Missing[0] != "nope" || rep.Missing[1] != "source" {
		t.Errorf("missing = %v", rep.Missing)
	}
}

func TestExpandEscapesMarkupInValues(t *testing.T) {
	meta := map[string]any{"title": "*x* <b>y</b> [z](http://evil)\nnext"}
	got, _ := Expand("{{page.title}}\n", meta)
	for _, want := range []string{`\*x\*`, `\<b\>`, `\[z\]`} {
		if !strings.Contains(got, want) {
			t.Errorf("%q not escaped in %q", want, got)
		}
	}
	if strings.Contains(got, "\nnext") {
		t.Errorf("a newline in a value broke the line: %q", got)
	}
}

func TestExpandInLinkTextAndEmphasis(t *testing.T) {
	got, _ := Expand("[{{page.title}}](a.md) and **{{page.author}}**\n", macroMeta)
	if got != "[Hello](a.md) and **Dave**\n" {
		t.Errorf("got %q", got)
	}
}

func TestExpandNoMacrosReturnsSourceUnchanged(t *testing.T) {
	src := "plain {text} and {{ other }} stuff\n"
	got, rep := Expand(src, macroMeta)
	if got != src || len(rep.Missing) != 0 {
		t.Errorf("got %q report %+v", got, rep)
	}
}

// --- Expand: what the browser gains from antennaApp ---

func TestExpandAllowsSpacesInsideTheBraces(t *testing.T) {
	got, _ := Expand("By {{ page.author }} and {{page.title }}.\n", macroMeta)
	if got != "By Dave and Hello.\n" {
		t.Errorf("got %q", got)
	}
}

func TestExpandWritesADateAsADate(t *testing.T) {
	meta := map[string]any{
		"day":  time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC),
		"when": time.Date(2026, 11, 19, 8, 30, 0, 0, time.UTC),
	}
	got, _ := Expand("{{page.day}} / {{page.when}}\n", meta)
	if got != "2026-11-19 / 2026-11-19T08:30:00Z\n" {
		t.Errorf("got %q", got)
	}
}

func TestExpandWritesNumbersAndBooleansAndSkipsNullsInLists(t *testing.T) {
	meta := map[string]any{"n": 3, "f": 1.5, "b": true, "list": []any{"a", nil, 2, map[string]any{"x": 1}, []any{"y"}}}
	got, rep := Expand("{{page.n}} {{page.f}} {{page.b}} {{page.list}}\n", meta)
	if got != "3 1.5 true a, 2\n" || len(rep.Missing) != 0 {
		t.Errorf("got %q report %+v", got, rep)
	}
}

func TestExpandMakesTheTitleALinkWhenThePageHasALink(t *testing.T) {
	meta := map[string]any{"title": "Hello", "author": "Dave", "link": "https://x.example/a?b=1&c=2"}
	got, rep := Expand("# {{page.title}}\n\n{{page.author}}\n", meta)
	if got != "# [Hello](https://x.example/a?b=1&c=2)\n\nDave\n" || len(rep.Warnings) != 0 {
		t.Errorf("got %q report %+v", got, rep)
	}
}

func TestExpandTitleLinkAddressesAreMadeSafeForMarkdown(t *testing.T) {
	meta := map[string]any{"title": "T", "link": "/a b/(c)"}
	got, _ := Expand("{{page.title}}\n", meta)
	if got != "[T](/a%20b/%28c%29)\n" {
		t.Errorf("got %q", got)
	}
}

func TestExpandTitleLinkRefusesScriptSchemes(t *testing.T) {
	meta := map[string]any{"title": "Hello", "link": "javascript:alert(1)"}
	got, rep := Expand("# {{page.title}}\n", meta)
	if got != "# Hello\n" || len(rep.Warnings) != 1 || !strings.Contains(rep.Warnings[0], "link") {
		t.Errorf("got %q report %+v", got, rep)
	}
}

func TestExpandTitleIsNotLinkedInsideALinkTheAuthorWrote(t *testing.T) {
	meta := map[string]any{"title": "X", "link": "https://x.example/"}
	got, _ := Expand("[Read {{page.title}}](/other)\n\n<https://y.example/> {{page.title}}\n", meta)
	want := "[Read X](/other)\n\n<https://y.example/> [X](https://x.example/)\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// --- MacroOption: the goldmark pipeline (ported from antennaApp macros_test.go) ---

// toHTML renders the body of src with macros expanded from its own front matter.
func toHTML(t *testing.T, src string, rep *Report) string {
	t.Helper()
	meta, body, err := Split([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	var opts []parser.Option
	if rep != nil {
		opts = append(opts, MacroOption(meta, rep))
	}
	md := goldmark.New(goldmark.WithParserOptions(opts...), goldmark.WithRendererOptions(html.WithUnsafe()))
	var buf bytes.Buffer
	if err := md.Convert(body, &buf); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func TestOptionExpandsPageKeys(t *testing.T) {
	var rep Report
	got := toHTML(t, "---\ntitle: Hello & Goodbye\nauthor: Dave\npub_date: 2026-11-19\nseriesNumber: 3\n---\n# {{page.title}}\n\nBy {{ page.author }} on {{page.pub_date}}, part {{page.seriesNumber}}.\n", &rep)
	for _, want := range []string{"<h1>Hello &amp; Goodbye</h1>", "By Dave on 2026-11-19, part 3."} {
		if !strings.Contains(got, want) {
			t.Errorf("%q not in:\n%s", want, got)
		}
	}
	if len(rep.Missing)+len(rep.Warnings) != 0 {
		t.Errorf("report: %+v", rep)
	}
}

func TestOptionLeavesCodeAlone(t *testing.T) {
	var rep Report
	got := toHTML(t, "---\ntitle: Hello\n---\nUse `{{page.title}}` here.\n\n```\n{{page.title}}\n```\n\n    {{page.title}}\n\n{{page.title}}\n", &rep)
	if strings.Count(got, "{{page.title}}") != 3 || strings.Count(got, "Hello") != 1 {
		t.Errorf("code expanded, or text not:\n%s", got)
	}
}

func TestOptionIsOnePass(t *testing.T) {
	var rep Report
	got := toHTML(t, "---\ntitle: \"{{page.author}}\"\nauthor: Dave\n---\n{{page.title}}\n", &rep)
	if strings.Contains(got, "Dave") || !strings.Contains(got, "{{page.author}}") {
		t.Errorf("a value was expanded again:\n%s", got)
	}
}

func TestOptionMissingKeyStaysVisibleAndIsReportedOnce(t *testing.T) {
	var rep Report
	got := toHTML(t, "---\ntitle: T\n---\n{{page.nope}} {{page.nope}} {{page.title}}\n", &rep)
	if !strings.Contains(got, "{{page.nope}} {{page.nope}} T") {
		t.Errorf("got:\n%s", got)
	}
	if len(rep.Missing) != 1 || rep.Missing[0] != "nope" {
		t.Errorf("missing = %v", rep.Missing)
	}
}

func TestOptionValuesCannotInjectMarkupOrHTML(t *testing.T) {
	var rep Report
	got := toHTML(t, "---\ntitle: \"<b>x</b> *y* [z](http://evil.example/)\"\n---\n{{page.title}}\n", &rep)
	for _, bad := range []string{"<b>", "<em>", "<a "} {
		if strings.Contains(got, bad) {
			t.Errorf("%q injected:\n%s", bad, got)
		}
	}
	if !strings.Contains(got, "&lt;b&gt;x&lt;/b&gt; *y* [z](http://evil.example/)") {
		t.Errorf("value not shown as text:\n%s", got)
	}
}

func TestOptionTitleBecomesALinkWhenLinkIsSet(t *testing.T) {
	var rep Report
	got := toHTML(t, "---\ntitle: Hello\nlink: https://x.example/a?b=1&c=2\n---\n# {{page.title}}\n", &rep)
	if !strings.Contains(got, `<h1><a href="https://x.example/a?b=1&amp;c=2">Hello</a></h1>`) {
		t.Errorf("got:\n%s", got)
	}
	got = toHTML(t, "---\ntitle: Hello\nauthor: Dave\nlink: https://x.example/\n---\n{{page.author}}\n", &rep)
	if strings.Contains(got, "<a ") {
		t.Errorf("author linked:\n%s", got)
	}
}

func TestOptionTitleLinkRefusesScriptSchemes(t *testing.T) {
	var rep Report
	got := toHTML(t, "---\ntitle: Hello\nlink: \"javascript:alert(1)\"\n---\n# {{page.title}}\n", &rep)
	if strings.Contains(got, "javascript:") || strings.Contains(got, "<a ") {
		t.Errorf("script link made:\n%s", got)
	}
	if !strings.Contains(got, "<h1>Hello</h1>") || len(rep.Warnings) != 1 || !strings.Contains(rep.Warnings[0], "link") {
		t.Errorf("got:\n%s\nreport: %+v", got, rep)
	}
}

func TestOptionIsOffUnlessAsked(t *testing.T) {
	if got := toHTML(t, "---\ntitle: T\n---\n{{page.title}}\n", nil); !strings.Contains(got, "{{page.title}}") {
		t.Errorf("expanded without being asked:\n%s", got)
	}
}

func TestOptionTitleLinkIsNotNestedInsideALink(t *testing.T) {
	var rep Report
	got := toHTML(t, "---\ntitle: X\nlink: https://x.example/\n---\n[Read {{page.title}}](/other)\n\n<https://y.example/> {{page.title}}\n", &rep)
	if !strings.Contains(got, `<a href="/other">Read X</a>`) {
		t.Errorf("the author's link should hold the plain value:\n%s", got)
	}
	if strings.Count(got, "<a ") != 3 {
		t.Errorf("want the author's two links and one title link, got:\n%s", got)
	}
}

func TestOptionReachesDottedKeysAndJoinsTouchingText(t *testing.T) {
	var rep Report
	got := toHTML(t, "---\nsource:\n  title: S\npub_date: 2026-11-19\n---\n{{page.source.title}} *{{page.pub_date}}*\n", &rep)
	if !strings.Contains(got, "S <em>2026-11-19</em>") {
		t.Errorf("got:\n%s", got)
	}
}

// A value that starts like a block (heading, list, rule) must not become one,
// wherever the macro sits on its line. cmarkbrowser's escapeValue missed these.
func TestExpandValuesCannotStartABlock(t *testing.T) {
	for _, tc := range []struct{ value, want string }{
		{"# x", `\# x`},
		{"- x", `\- x`},
		{"+ x", `\+ x`},
		{"1. x", `1\. x`},
		{"2) x", `2\) x`},
		{"=== x", `\=== x`},
		{"a-b # c", "a-b # c"}, // only a leading character matters
	} {
		got, _ := Expand("{{page.v}}\n", map[string]any{"v": tc.value})
		if got != tc.want+"\n" {
			t.Errorf("value %q: got %q, want %q", tc.value, got, tc.want+"\n")
		}
	}
	// and in a list item or quote, where the block would nest
	got, _ := Expand("- {{page.v}}\n> {{page.v}}\n", map[string]any{"v": "- x"})
	if got != "- \\- x\n> \\- x\n" {
		t.Errorf("got %q", got)
	}
}
