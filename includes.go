package frontmatter

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

const (
	// includeWarnDepth is the nesting level above which an include draws a
	// warning; the document is level 0 and the files it includes are level 1.
	includeWarnDepth = 3
	// includeMaxDepth is the nesting level above which an include is an error.
	includeMaxDepth = 5
)

/** ErrCycle is wrapped by the error Includes returns when a file includes
 * itself, directly or through others. The message names the chain.
 *
 * Example:
 *   _, _, err := frontmatter.Includes(text, fsys, frontmatter.Markdown)
 *   errors.Is(err, frontmatter.ErrCycle)
 */
var ErrCycle = errors.New("include cycle")

/** ErrTooDeep is wrapped by the error Includes returns when text includes are
 * nested more than five levels deep. The message names the chain.
 *
 * Example:
 *   errors.Is(err, frontmatter.ErrTooDeep)
 */
var ErrTooDeep = errors.New("includes nested too deep")

/** Format says how one kind of document spells its include directives and how
 * an included code file is written out. The engine in Includes knows nothing
 * else about the format.
 *
 * Fields:
 *   Name (string) — for messages
 *   TextDirective (string) — the word that includes a text file, such as "@include-text-block"; "" for none
 *   CodeDirective (string) — the word that includes a code file; "" for none
 *   Fence (func(content, lang string) (string, error)) — writes an included code file, or says why it cannot (the directive is then left as written); required when CodeDirective is set
 *   Verbatim (func(text string) [][2]int) — byte ranges of text in which directives are not expanded, such as code fences; may be nil
 */
type Format struct {
	Name          string
	TextDirective string
	CodeDirective string
	Fence         func(content, lang string) (string, error)
	Verbatim      func(text string) [][2]int
}

/** Markdown is the Format for CommonMark: @include-text-block FILE and
 * @include-code-block FILE [LANGUAGE]. A code file is written as ~~~LANGUAGE
 * ... ~~~, always three tildes, the closing fence at the start of its line. A
 * file with a line that would close that fence early (nothing but three or more
 * tildes, up to three spaces of indent) is not included: the directive is left
 * as written and a warning says to indent the example in that file. A directive
 * inside a fenced code block is left alone, so a page can show the syntax.
 */
var Markdown = Format{
	Name:          "Markdown",
	TextDirective: "@include-text-block",
	CodeDirective: "@include-code-block",
	Fence:         tildeFence,
	Verbatim:      fencedCode,
}

// tildeFence writes content as a ~~~ fence with an optional language. It fails
// when a line of content would close the fence: per CommonMark a closing fence is
// at most three spaces of indent, then three or more tildes, then only white
// space. An opening line such as "~~~shell" closes nothing.
func tildeFence(content, lang string) (string, error) {
	for i, line := range strings.Split(content, "\n") {
		if closesTildeFence(line) {
			return "", fmt.Errorf("line %d would close the ~~~ fence early; indent the example in that file", i+1)
		}
	}
	return "~~~" + lang + "\n" + strings.TrimRight(content, "\n") + "\n~~~", nil
}

// closesTildeFence reports whether line could close a tilde fence.
func closesTildeFence(line string) bool {
	line = strings.TrimRight(line, " \t\r")
	indent := len(line) - len(strings.TrimLeft(line, " "))
	rest := line[indent:]
	return indent <= 3 && len(rest) >= 3 && strings.Trim(rest, "~") == ""
}

// fencedCode returns the byte ranges of the content lines of the fenced code
// blocks of a Markdown document.
func fencedCode(src string) [][2]int {
	b := []byte(src)
	doc := goldmark.New().Parser().Parse(text.NewReader(b))
	var spans [][2]int
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if fc, ok := n.(*ast.FencedCodeBlock); ok && entering {
			if lines := fc.Lines(); lines.Len() > 0 {
				spans = append(spans, [2]int{lines.At(0).Start, lines.At(lines.Len() - 1).Stop})
			}
		}
		return ast.WalkContinue, nil
	})
	return spans
}

/** Included says what Includes did besides changing the text.
 *
 * Fields:
 *   Files ([]string) — every file a directive named, nested ones too, once each, in the order first met; a missing file is listed, a refused path is not
 *   Warnings ([]string) — nesting past three levels, and the reason a directive was left as written
 */
type Included struct {
	Files    []string
	Warnings []string
}

/** Includes expands the include directives of text, reading files through fsys.
 * A directive starts its line (up to three spaces of indent) and is followed by
 * a file name. A text file is itself expanded, so its own directives are
 * followed; a code file is code and is never expanded. An included file's front
 * matter is dropped (a block that is not valid YAML is not front matter and is
 * kept). A name that is not a valid fs path (absolute, or leaving the root with
 * "..") is refused; a file that is missing, a directory or unreadable is left as
 * written; both are reported in Warnings. Nesting deeper than three levels draws
 * a warning and expansion goes on; deeper than five, or a cycle, is an error
 * naming the chain, and the text is returned as it was.
 *
 * Parameters:
 *   text (string) — the document body
 *   fsys (fs.FS) — the site root
 *   f (Format) — how the document spells directives
 *
 * Returns:
 *   string — the text with directives expanded
 *   Included — the files named and the warnings
 *   error — wraps ErrCycle or ErrTooDeep
 *
 * Example:
 *   out, inc, err := frontmatter.Includes("Top.\n\n@include-text-block a.txt\n", os.DirFS("."), frontmatter.Markdown)
 */
func Includes(text string, fsys fs.FS, f Format) (string, Included, error) {
	words := []string{}
	for _, w := range []string{f.TextDirective, f.CodeDirective} {
		if w != "" {
			words = append(words, regexp.QuoteMeta(w))
		}
	}
	if len(words) == 0 {
		return text, Included{}, nil
	}
	st := &includeRun{
		fsys: fsys,
		f:    f,
		seen: map[string]bool{},
		re: regexp.MustCompile(`(?m)(?:\A|\s*\n)[ ]{0,3}(` + strings.Join(words, "|") +
			`)[ \t]+(\S+)(?:[ \t]+(\w+))?[ \t]*$`),
	}
	out := st.expand(text, nil)
	if st.err != nil {
		return text, st.inc, st.err
	}
	return out, st.inc, nil
}

// includeRun is the state of one call of Includes.
type includeRun struct {
	fsys fs.FS
	f    Format
	re   *regexp.Regexp
	seen map[string]bool
	inc  Included
	err  error
}

func (st *includeRun) warnf(format string, args ...any) {
	st.inc.Warnings = append(st.inc.Warnings, fmt.Sprintf(format, args...))
}

// expand expands the directives of one document. chain holds the included files
// above it (empty for the document itself).
func (st *includeRun) expand(text string, chain []string) string {
	if st.err != nil || !st.re.MatchString(text) {
		return text
	}
	level := len(chain) + 1 // the level of the files this document includes
	var verbatim [][2]int
	if st.f.Verbatim != nil {
		verbatim = st.f.Verbatim(text)
	}
	var sb strings.Builder
	pos := 0
	for _, m := range st.re.FindAllStringSubmatchIndex(text, -1) {
		if st.err != nil {
			break
		}
		if inSpans(verbatim, m[2]) {
			continue // the directive is shown, not run
		}
		word, name, lang := text[m[2]:m[3]], text[m[4]:m[5]], ""
		if m[6] >= 0 {
			lang = text[m[6]:m[7]]
		}
		kind := "text"
		if word == st.f.CodeDirective && st.f.CodeDirective != "" {
			kind = "code"
		}
		lead := "\n\n"
		if m[0] == 0 {
			lead = "" // nothing but blank lines come before it: no blank line is added
		}
		repl, ok := st.include(kind, word, name, lang, lead, chain, level)
		if !ok {
			continue // left as written
		}
		sb.WriteString(text[pos:m[0]])
		sb.WriteString(repl)
		pos = m[1]
	}
	if st.err != nil {
		return text
	}
	sb.WriteString(text[pos:])
	return sb.String()
}

// include reads one file and returns what replaces its directive; ok is false
// when the directive is to be left as written.
func (st *includeRun) include(kind, word, name, lang, lead string, chain []string, level int) (string, bool) {
	clean := path.Clean(name)
	if !fs.ValidPath(clean) || strings.HasPrefix(name, "/") {
		st.warnf("%s: %s attempts to leave the site, left as written", word, name)
		return "", false
	}
	if !st.seen[clean] {
		st.seen[clean] = true
		st.inc.Files = append(st.inc.Files, clean)
	}
	info, err := fs.Stat(st.fsys, clean)
	if err != nil {
		st.warnf("%s: %s: %v, left as written", word, clean, err)
		return "", false
	}
	if info.IsDir() {
		st.warnf("%s: %s is a directory, left as written", word, clean)
		return "", false
	}
	src, err := fs.ReadFile(st.fsys, clean)
	if err != nil {
		st.warnf("%s: %s: %v, left as written", word, clean, err)
		return "", false
	}
	if kind == "code" {
		if st.f.Fence == nil {
			st.warnf("%s: %s: this format has no code includes, left as written", word, clean)
			return "", false
		}
		fenced, err := st.f.Fence(string(src), lang)
		if err != nil {
			st.warnf("%s: %s: %v, left as written", word, clean, err)
			return "", false
		}
		return lead + fenced, true
	}
	for _, c := range chain {
		if c == clean {
			st.err = fmt.Errorf("%w: %s", ErrCycle, strings.Join(append(append([]string{}, chain...), clean), " -> "))
			return "", false
		}
	}
	next := append(append([]string{}, chain...), clean)
	if level > includeMaxDepth {
		st.err = fmt.Errorf("%w: more than %d levels deep: %s", ErrTooDeep, includeMaxDepth, strings.Join(next, " -> "))
		return "", false
	}
	if level > includeWarnDepth {
		st.warnf("include nested %d levels deep (more than %d is an error): %s", level, includeMaxDepth, strings.Join(next, " -> "))
	}
	body := src
	if _, b, err := Split(src); err == nil {
		body = b
	}
	return lead + strings.TrimRight(st.expand(string(body), next), "\n"), true
}

// inSpans reports whether offset i lies in one of the spans.
func inSpans(spans [][2]int, i int) bool {
	for _, s := range spans {
		if i >= s[0] && i < s[1] {
			return true
		}
	}
	return false
}
