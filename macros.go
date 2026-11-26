package frontmatter

import (
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// macroPattern matches {{page.KEY}}, with optional spaces inside the braces as
// Jekyll allows. KEY is letters, digits, underscore, hyphen and dots (a dot
// reaches into a nested table).
var macroPattern = regexp.MustCompile(`\{\{\s*page\.([A-Za-z0-9_.-]+)\s*\}\}`)

/** Report says what went wrong while macros were expanded. Neither field makes
 * the expansion fail: a macro that cannot be expanded is left as written.
 *
 * Fields:
 *   Missing ([]string) — keys with no usable value (absent, null or a table), once each, in the order met
 *   Warnings ([]string) — other problems, such as a link value that is not a safe address
 */
type Report struct {
	Missing  []string
	Warnings []string
}

// expansion holds what one expansion of one page needs.
type expansion struct {
	meta       map[string]any
	rep        *Report
	seenMiss   map[string]bool
	linkChecks bool   // the title link has been looked at
	linkDest   string // the address the title links to, "" for none
}

func newExpansion(meta map[string]any, rep *Report) *expansion {
	if rep == nil {
		rep = &Report{}
	}
	return &expansion{meta: meta, rep: rep, seenMiss: map[string]bool{}}
}

// value returns the text a macro key expands to. A key that has none is
// reported once and ok is false.
func (e *expansion) value(key string) (string, bool) {
	if s, ok := lookupText(e.meta, key); ok {
		return s, true
	}
	if !e.seenMiss[key] {
		e.seenMiss[key] = true
		e.rep.Missing = append(e.rep.Missing, key)
	}
	return "", false
}

// titleLink returns the address the expanded title links to, or "" when the
// page has no usable link. A link that is not a safe address is reported once.
func (e *expansion) titleLink() string {
	if e.linkChecks {
		return e.linkDest
	}
	e.linkChecks = true
	raw, ok := e.meta["link"].(string)
	raw = strings.TrimSpace(raw)
	if !ok || raw == "" {
		return ""
	}
	if !safeAddress(raw) {
		e.rep.Warnings = append(e.rep.Warnings, fmt.Sprintf("ignoring link %q: only http, https, mailto and relative addresses make the title a link", raw))
		return ""
	}
	e.linkDest = raw
	return raw
}

// safeAddress reports whether raw can be the address of a link that front
// matter supplies: http, https, mailto or relative. Script and data schemes are
// refused.
func safeAddress(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "" || u.Scheme == "http" || u.Scheme == "https" || u.Scheme == "mailto")
}

// lookupText finds a dotted key in front matter and returns its text. ok is
// false for an absent key, a null, a table, or a value that is not text, a
// number, a boolean, a date or a list of those.
func lookupText(meta map[string]any, key string) (string, bool) {
	var cur any = meta
	for _, part := range strings.Split(key, ".") {
		m, isMap := cur.(map[string]any)
		if !isMap {
			return "", false
		}
		cur, isMap = m[part]
		if !isMap {
			return "", false
		}
	}
	return scalarText(cur)
}

// scalarText returns the text of a scalar or of a list of scalars.
func scalarText(v any) (string, bool) {
	switch v := v.(type) {
	case string:
		return v, true
	case bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return fmt.Sprint(v), true
	case time.Time:
		if v.Hour() == 0 && v.Minute() == 0 && v.Second() == 0 && v.Nanosecond() == 0 {
			return v.Format("2006-01-02"), true
		}
		return v.Format(time.RFC3339), true
	case []any:
		var parts []string
		for _, item := range v {
			if _, isList := item.([]any); isList {
				continue
			}
			if s, ok := scalarText(item); ok {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, ", "), true
	}
	return "", false
}

/** Expand replaces {{page.KEY}} macros in Markdown source with values from meta
 * and returns Markdown, the way BonBon pages use them (a story starts with
 * "# {{page.title}}"). It expands only in text: never in code spans, fenced
 * code or indented code. It makes one pass, so a value that itself holds a
 * macro is shown, not expanded. A value is escaped so it cannot become markup,
 * a link, HTML, or the start of a heading or list. Spaces are allowed inside the
 * braces. A dotted key reaches into a nested table ({{page.source.title}}); a
 * list shows its items joined with commas; a date shows as 2026-11-19, or in RFC
 * 3339 when it has a time. A key that is absent, null or a table leaves the macro
 * visible and is reported. When the page has a link value, the expanded title is
 * a link to it, unless the macro already sits inside a link; an address that is
 * not http, https, mailto or relative is ignored and reported as a warning.
 *
 * Parameters:
 *   src (string) — the Markdown body
 *   meta (map[string]any) — the page's front matter
 *
 * Returns:
 *   string — src with its macros expanded; src itself when it has none
 *   Report — the keys that could not be expanded and any other warnings
 *
 * Example:
 *   out, rep := frontmatter.Expand("# {{page.title}}", map[string]any{"title": "Hi"}) // "# Hi"
 */
func Expand(src string, meta map[string]any) (string, Report) {
	if !macroPattern.MatchString(src) {
		return src, Report{}
	}
	e := newExpansion(meta, nil)
	b := []byte(src)
	doc := goldmark.New().Parser().Parse(text.NewReader(b))
	type edit struct {
		start, stop int
		repl        string
	}
	var edits []edit
	done := map[ast.Node]bool{}

	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch v := n.(type) {
		case *ast.CodeSpan:
			return ast.WalkSkipChildren, nil
		case *ast.Text:
			if done[v] {
				return ast.WalkContinue, nil
			}
			// goldmark splits a run of text at characters such as _ and *, so
			// join the touching text nodes and look for macros in the run.
			start, stop := v.Segment.Start, v.Segment.Stop
			for s := v.NextSibling(); s != nil; s = s.NextSibling() {
				t, ok := s.(*ast.Text)
				if !ok || t.Segment.Start != stop {
					break
				}
				done[t] = true
				stop = t.Segment.Stop
			}
			for _, m := range macroPattern.FindAllSubmatchIndex(b[start:stop], -1) {
				key := string(b[start+m[2] : start+m[3]])
				val, ok := e.value(key)
				if !ok {
					continue
				}
				repl := escapeValue(val)
				if key == "title" && !insideLink(v) {
					if dest := e.titleLink(); dest != "" {
						repl = "[" + repl + "](" + escapeDest(dest) + ")"
					}
				}
				edits = append(edits, edit{start + m[0], start + m[1], repl})
			}
		}
		return ast.WalkContinue, nil
	})
	if len(edits) == 0 {
		return src, *e.rep
	}
	sort.Slice(edits, func(i, j int) bool { return edits[i].start < edits[j].start })
	var sb strings.Builder
	sb.Grow(len(src))
	pos := 0
	for _, ed := range edits {
		sb.Write(b[pos:ed.start])
		sb.WriteString(ed.repl)
		pos = ed.stop
	}
	sb.Write(b[pos:])
	return sb.String(), *e.rep
}

// leadingNumber matches the start of a value that would open an ordered list
// item: one to nine digits, then "." or ")", then a space or the end.
var leadingNumber = regexp.MustCompile(`^([0-9]{1,9})([.)])( |$)`)

// escapeValue makes a front matter value inert as Markdown: one line, with
// every character that could start markup, a link, HTML or a macro escaped,
// and a leading character that could open a heading, list, quote or rule.
func escapeValue(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	var sb strings.Builder
	if m := leadingNumber.FindStringSubmatchIndex(s); m != nil {
		sb.WriteString(s[:m[3]]) // the digits
		sb.WriteByte('\\')
		s = s[m[3]:] // from the "." or ")"
	} else if s != "" && strings.ContainsRune("#+-=", rune(s[0])) {
		sb.WriteByte('\\')
	}
	for _, r := range s {
		switch r {
		case '\\', '`', '*', '_', '[', ']', '<', '>', '&', '~', '|', '{', '}', '!':
			sb.WriteByte('\\')
		}
		sb.WriteRune(r)
	}
	return sb.String()
}

// escapeDest makes an address safe to write as a Markdown link destination.
func escapeDest(s string) string {
	return strings.NewReplacer(" ", "%20", "(", "%28", ")", "%29", "<", "%3C", ">", "%3E").Replace(s)
}

// insideLink reports whether n sits inside a link or autolink.
func insideLink(n ast.Node) bool {
	for p := n.Parent(); p != nil; p = p.Parent() {
		switch p.(type) {
		case *ast.Link, *ast.AutoLink:
			return true
		}
	}
	return false
}

/** MacroOption returns a goldmark parser option that expands {{page.KEY}}
 * macros in the parsed document, for a program that renders to HTML. It works
 * on the document's text nodes, never on the source text, so a value is
 * inserted as escaped text and can never become Markdown, a link or HTML. The
 * rules are those of Expand: code is never touched, one pass, a missing key
 * stays visible and is reported, the title links to the page's link value.
 *
 * Parameters:
 *   meta (map[string]any) — the page's front matter
 *   report (*Report) — receives what could not be expanded; may be nil
 *
 * Returns:
 *   parser.Option — pass to goldmark.WithParserOptions
 *
 * Example:
 *   md := goldmark.New(goldmark.WithParserOptions(frontmatter.MacroOption(meta, &rep)))
 */
func MacroOption(meta map[string]any, report *Report) parser.Option {
	return parser.WithASTTransformers(util.Prioritized(macroTransformer{newExpansion(meta, report)}, 50))
}

// macroTransformer is the goldmark AST transformer behind MacroOption.
type macroTransformer struct {
	e *expansion
}

// Transform expands the macros in the text nodes of doc.
func (t macroTransformer) Transform(doc *ast.Document, reader text.Reader, _ parser.Context) {
	src := reader.Source()
	var texts []*ast.Text
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		switch v := n.(type) {
		case *ast.CodeSpan:
			return ast.WalkSkipChildren, nil
		case *ast.Text:
			if entering {
				texts = append(texts, v)
			}
		}
		return ast.WalkContinue, nil
	})
	merged := map[*ast.Text]bool{}
	for _, n := range texts {
		if merged[n] {
			continue
		}
		// goldmark splits a run of text at characters such as _ and *: join
		// touching text nodes when the joined text holds a macro, so a key
		// such as pub_date is found whole.
		run := []*ast.Text{n}
		stop := n.Segment.Stop
		for last := n; !last.SoftLineBreak() && !last.HardLineBreak() && !last.IsRaw(); {
			next, ok := last.NextSibling().(*ast.Text)
			if !ok || next.Segment.Start != stop || next.IsRaw() {
				break
			}
			run = append(run, next)
			stop = next.Segment.Stop
			last = next
		}
		if len(run) > 1 && macroPattern.Match(src[n.Segment.Start:stop]) {
			last := run[len(run)-1]
			n.Segment = n.Segment.WithStop(stop)
			n.SetSoftLineBreak(last.SoftLineBreak())
			n.SetHardLineBreak(last.HardLineBreak())
			for _, extra := range run[1:] {
				merged[extra] = true
				extra.Parent().RemoveChild(extra.Parent(), extra)
			}
		}
		t.expandText(n, src)
	}
}

// expandText replaces the macros in one text node with their values.
func (t macroTransformer) expandText(n *ast.Text, src []byte) {
	seg := n.Segment
	matches := macroPattern.FindAllSubmatchIndex(src[seg.Start:seg.Stop], -1)
	if len(matches) == 0 {
		return
	}
	parent := n.Parent()
	insert := func(node ast.Node) { parent.InsertBefore(parent, n, node) }
	pos := seg.Start
	for _, m := range matches {
		key := string(src[seg.Start+m[2] : seg.Start+m[3]])
		val, ok := t.e.value(key)
		if !ok {
			continue // stays in the surrounding text
		}
		if before := seg.Start + m[0]; before > pos {
			insert(ast.NewTextSegment(text.NewSegment(pos, before)))
		}
		pos = seg.Start + m[1]
		str := ast.NewString([]byte(val))
		str.SetRaw(true) // written HTML-escaped, never parsed or entity-resolved
		// A title inside a link the author wrote stays plain text: nested links
		// are not valid HTML.
		if dest := t.titleDest(key, n); dest != "" {
			link := ast.NewLink()
			link.Destination = []byte(dest)
			link.AppendChild(link, str)
			insert(link)
		} else {
			insert(str)
		}
	}
	if pos == seg.Start {
		return // nothing was expanded
	}
	tail := ast.NewTextSegment(text.NewSegment(pos, seg.Stop))
	tail.SetSoftLineBreak(n.SoftLineBreak())
	tail.SetHardLineBreak(n.HardLineBreak())
	insert(tail)
	parent.RemoveChild(parent, n)
}

// titleDest returns the address the expanded title links to, or "" when key is
// not the title, n is inside a link, or the page has no usable link.
func (t macroTransformer) titleDest(key string, n ast.Node) string {
	if key != "title" || insideLink(n) {
		return ""
	}
	return t.e.titleLink()
}
