package frontmatter

import (
	"errors"
	"strings"
	"testing"
	"testing/fstest"
)

func files(m map[string]string) fstest.MapFS {
	fsys := fstest.MapFS{}
	for k, v := range m {
		fsys[k] = &fstest.MapFile{Data: []byte(v)}
	}
	return fsys
}

// --- ported from antennaApp include_nested_test.go (DR-0024 12) ---

func TestIncludesNestExpandAndAreReportedInOrder(t *testing.T) {
	fsys := files(map[string]string{
		"a.txt": "A start\n\n@include-text-block b.txt\n\nA end\n",
		"b.txt": "B start\n\n@include-text-block c.txt\n\nB end\n",
		"c.txt": "C only\n",
		"d.txt": "D only\n",
	})
	src := "Top.\n\n@include-text-block a.txt\n\nMiddle.\n\n@include-text-block d.txt\n\n@include-text-block c.txt\n"
	out, inc, err := Includes(src, fsys, Markdown)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"A start", "B start", "C only", "B end", "A end", "D only"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "@include-") {
		t.Errorf("a directive is left:\n%s", out)
	}
	if got := strings.Join(inc.Files, ","); got != "a.txt,b.txt,c.txt,d.txt" {
		t.Errorf("files (each once, first-met order): %s", got)
	}
	if len(inc.Warnings) != 0 {
		t.Errorf("three levels or fewer draw no warning: %v", inc.Warnings)
	}
}

func chainOf(n int) map[string]string {
	m := map[string]string{}
	for i := 1; i <= n; i++ {
		body := "level " + string(rune('0'+i)) + "\n"
		if i < n {
			body += "\n@include-text-block l" + string(rune('0'+i+1)) + ".txt\n"
		}
		m["l"+string(rune('0'+i))+".txt"] = body
	}
	return m
}

func TestIncludesWarnFromTheFourthLevelAndStopAtTheSixth(t *testing.T) {
	// The page is level 0; l1 is level 1 ... l5 is level 5.
	out, inc, err := Includes("Top.\n\n@include-text-block l1.txt\n", files(chainOf(5)), Markdown)
	if err != nil {
		t.Fatalf("five levels are allowed: %s", err)
	}
	if !strings.Contains(out, "level 5") || len(inc.Files) != 5 {
		t.Errorf("all five levels should be in:\n%s\n%v", out, inc.Files)
	}
	if len(inc.Warnings) != 2 {
		t.Fatalf("one warning each for level 4 and level 5, got %v", inc.Warnings)
	}
	if !strings.Contains(inc.Warnings[0], "l4.txt") || !strings.Contains(inc.Warnings[1], "l5.txt") {
		t.Errorf("a warning names the file: %v", inc.Warnings)
	}

	_, _, err = Includes("Top.\n\n@include-text-block l1.txt\n", files(chainOf(6)), Markdown)
	if !errors.Is(err, ErrTooDeep) {
		t.Fatalf("six levels: err = %v, want ErrTooDeep", err)
	}
	if !strings.Contains(err.Error(), "l6.txt") || !strings.Contains(err.Error(), "l1.txt") {
		t.Errorf("the error names the chain: %v", err)
	}
}

func TestIncludeCycleIsAnError(t *testing.T) {
	fsys := files(map[string]string{
		"a.txt":    "A\n\n@include-text-block b.txt\n",
		"b.txt":    "B\n\n@include-text-block a.txt\n",
		"self.txt": "S\n\n@include-text-block self.txt\n",
	})
	_, _, err := Includes("Top.\n\n@include-text-block a.txt\n", fsys, Markdown)
	if !errors.Is(err, ErrCycle) || !strings.Contains(err.Error(), "a.txt") || !strings.Contains(err.Error(), "b.txt") {
		t.Errorf("a cycle: err = %v", err)
	}
	_, _, err = Includes("Top.\n\n@include-text-block self.txt\n", fsys, Markdown)
	if !errors.Is(err, ErrCycle) {
		t.Errorf("a file that includes itself: err = %v", err)
	}
}

func TestIncludedCodeIsNeverExpanded(t *testing.T) {
	fsys := files(map[string]string{
		"sample.txt": "write @include-text-block other.txt like this\n\n@include-text-block other.txt\n",
		"other.txt":  "OTHER\n",
	})
	out, inc, err := Includes("Top.\n\n@include-code-block sample.txt text\n", fsys, Markdown)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "OTHER") || !strings.Contains(out, "@include-text-block other.txt") {
		t.Errorf("the sample's directives stay text:\n%s", out)
	}
	if len(inc.Files) != 1 || inc.Files[0] != "sample.txt" {
		t.Errorf("only the code file is listed: %v", inc.Files)
	}
}

func TestIncludedBlocksAreSeparatedFromTheTextAround(t *testing.T) {
	fsys := files(map[string]string{"hello.go": "package main\n", "note.txt": "A note.\n"})
	src := "Intro line.\n\n@include-code-block hello.go go\n\nMiddle line.\n\n@include-text-block note.txt\n\nLast line.\n"
	out, _, err := Includes(src, fsys, Markdown)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "~~~") && !strings.HasPrefix(line, "~~~") {
			t.Errorf("a fence must start its line: %q\n%s", line, out)
		}
	}
	if !strings.Contains(out, "Intro line.\n\n~~~go\npackage main\n~~~\n") {
		t.Errorf("a blank line before the fence, none inside it:\n%q", out)
	}
	if !strings.Contains(out, "Middle line.\n\nA note.\n") || !strings.Contains(out, "A note.\n\nLast line.") {
		t.Errorf("blank lines around an included text block:\n%q", out)
	}
}

func TestAMissingOrRefusedFileIsLeftAsWrittenAndReported(t *testing.T) {
	out, inc, err := Includes("Top.\n\n@include-text-block gone.txt\n\n@include-text-block ../x.txt\n", files(nil), Markdown)
	if err != nil {
		t.Fatalf("a missing file is left in the text, not an error: %s", err)
	}
	if !strings.Contains(out, "@include-text-block gone.txt") || !strings.Contains(out, "@include-text-block ../x.txt") {
		t.Errorf("left as written:\n%s", out)
	}
	if len(inc.Files) != 1 || inc.Files[0] != "gone.txt" {
		t.Errorf("a missing file is listed, a refused path is not: %v", inc.Files)
	}
	all := strings.Join(inc.Warnings, "\n")
	if !strings.Contains(all, "gone.txt") || !strings.Contains(all, "../x.txt") {
		t.Errorf("both problems are reported: %v", inc.Warnings)
	}
}

func TestAnIncludedFilesFrontMatterIsDropped(t *testing.T) {
	fsys := files(map[string]string{"frag.txt": "---\ntitle: Fragment\n---\nBody of fragment.\n"})
	out, _, err := Includes("Top.\n\n@include-text-block frag.txt\n", fsys, Markdown)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "title: Fragment") || !strings.Contains(out, "Body of fragment.") {
		t.Errorf("front matter dropped, body kept:\n%s", out)
	}
}

// --- new ---

func TestAFragmentThatIsNotFrontMatterIsKeptWhole(t *testing.T) {
	// A block that does not parse as YAML is not front matter (antennaApp's rule).
	fsys := files(map[string]string{"rules.txt": "---\nsome prose\n---\nmore\n"})
	out, _, _ := Includes("@include-text-block rules.txt\n", fsys, Markdown)
	if !strings.Contains(out, "some prose") {
		t.Errorf("text between two rules was dropped:\n%s", out)
	}
}

func TestADirectiveAtTheStartOfTheDocumentIsExpanded(t *testing.T) {
	out, _, _ := Includes("@include-text-block a.txt\n\nAfter.\n", files(map[string]string{"a.txt": "A\n"}), Markdown)
	if out != "A\n\nAfter.\n" {
		t.Errorf("got %q", out)
	}
}

func TestTextWithNoDirectiveIsReturnedAsItWas(t *testing.T) {
	src := "Just text, and a mention of @include elsewhere.\n"
	out, inc, err := Includes(src, files(nil), Markdown)
	if out != src || err != nil || len(inc.Files) != 0 || len(inc.Warnings) != 0 {
		t.Errorf("got %q %+v %v", out, inc, err)
	}
}

func TestAFileNamedTwiceIsListedOnce(t *testing.T) {
	_, inc, _ := Includes("@include-text-block a.txt\n\n@include-text-block a.txt\n", files(map[string]string{"a.txt": "A\n"}), Markdown)
	if len(inc.Files) != 1 {
		t.Errorf("files: %v", inc.Files)
	}
}

func TestOnlyPathsInsideTheFileSystemAreRead(t *testing.T) {
	fsys := files(map[string]string{"a.txt": "A\n", "sub/b.txt": "B\n"})
	for _, name := range []string{"/etc/passwd", "../a.txt", "sub/../../a.txt", "..", "C:/x"} {
		out, inc, _ := Includes("@include-text-block "+name+"\n", fsys, Markdown)
		if !strings.Contains(out, "@include-text-block "+name) {
			t.Errorf("%q was read:\n%s", name, out)
		}
		if name != "C:/x" && len(inc.Files) != 0 {
			t.Errorf("%q was listed: %v", name, inc.Files)
		}
	}
	// a path that cleans to a valid one is fine
	out, _, _ := Includes("@include-text-block sub/../a.txt\n\n@include-text-block ./sub/b.txt\n", fsys, Markdown)
	if out != "A\n\nB\n" {
		t.Errorf("got %q", out)
	}
}

func TestADirectoryIsLeftAsWritten(t *testing.T) {
	out, _, err := Includes("@include-text-block sub\n", files(map[string]string{"sub/b.txt": "B\n"}), Markdown)
	if err != nil || !strings.Contains(out, "@include-text-block sub") {
		t.Errorf("got %q, %v", out, err)
	}
}

func TestDirectivesInsideFencedCodeAreLeftAlone(t *testing.T) {
	fsys := files(map[string]string{"a.txt": "A\n", "b.txt": "B\n", "c.txt": "C\n"})
	src := "Syntax:\n\n```\n@include-text-block a.txt\n```\n\n~~~md\n@include-code-block b.txt go\n~~~\n\n@include-text-block c.txt\n"
	out, inc, err := Includes(src, fsys, Markdown)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "```\n@include-text-block a.txt\n```") || !strings.Contains(out, "~~~md\n@include-code-block b.txt go\n~~~") {
		t.Errorf("a directive in a fence was expanded:\n%s", out)
	}
	if !strings.Contains(out, "\n\nC\n") || strings.Contains(out, "@include-text-block c.txt") {
		t.Errorf("a directive after the fence was not expanded:\n%s", out)
	}
	if len(inc.Files) != 1 || inc.Files[0] != "c.txt" {
		t.Errorf("only the real include is listed: %v", inc.Files)
	}
}

func TestAnUnclosedFenceHidesTheRestOfTheDocument(t *testing.T) {
	out, _, _ := Includes("```\n@include-text-block a.txt\n", files(map[string]string{"a.txt": "A\n"}), Markdown)
	if !strings.Contains(out, "@include-text-block a.txt") {
		t.Errorf("got %q", out)
	}
}

func TestFencesInTheIncludedTextCountToo(t *testing.T) {
	fsys := files(map[string]string{"doc.txt": "How:\n\n```\n@include-text-block other.txt\n```\n", "other.txt": "OTHER\n"})
	out, _, _ := Includes("@include-text-block doc.txt\n", fsys, Markdown)
	if strings.Contains(out, "OTHER") {
		t.Errorf("a directive in an included file's fence was expanded:\n%s", out)
	}
}

// An included code file is written as ~~~LANG ... ~~~, always three tildes. A line
// that would close that fence early makes the directive stay as written, with a
// warning, so the output never changes shape (author, 2026-11-26).
func TestACodeFileIsAlwaysFencedWithThreeTildes(t *testing.T) {
	fsys := files(map[string]string{
		"plain.go":    "package main\n",
		"opening.md":  "Opens only:\n\n~~~shell\nls\n",            // an opening line closes nothing
		"indented.md": "Example:\n\n    ~~~\n    code\n    ~~~\n", // indented four: text, not a fence
		"backtick.md": "Example:\n\n```\ncode\n```\n",             // a different fence character
	})
	for _, tc := range []struct{ file, lang string }{{"plain.go", "go"}, {"opening.md", "markdown"}, {"indented.md", "markdown"}, {"backtick.md", "markdown"}} {
		out, inc, err := Includes("@include-code-block "+tc.file+" "+tc.lang+"\n", fsys, Markdown)
		if err != nil || len(inc.Warnings) != 0 {
			t.Errorf("%s: err = %v, warnings = %v", tc.file, err, inc.Warnings)
		}
		if !strings.HasPrefix(out, "~~~"+tc.lang+"\n") || !strings.HasSuffix(out, "\n~~~\n") || strings.HasPrefix(out, "~~~~") || strings.Contains(out, "\n~~~~") {
			t.Errorf("%s: want a three tilde fence, got:\n%s", tc.file, out)
		}
	}
}

func TestACodeFileWithALineThatClosesTheFenceIsLeftAsWritten(t *testing.T) {
	for name, content := range map[string]string{
		"bare":            "Example:\n\n~~~shell\nls\n~~~\n\nend\n",
		"three-indent":    "x\n   ~~~\ny\n",
		"four-tildes":     "x\n~~~~\ny\n",
		"trailing-spaces": "x\n~~~   \ny\n",
		"crlf":            "x\r\n~~~\r\ny\r\n",
	} {
		fsys := files(map[string]string{"sample.md": content})
		src := "Top.\n\n@include-code-block sample.md markdown\n"
		out, inc, err := Includes(src, fsys, Markdown)
		if err != nil {
			t.Errorf("%s: %v", name, err)
		}
		if out != src {
			t.Errorf("%s: the directive should be left as written, got:\n%s", name, out)
		}
		if len(inc.Files) != 1 || inc.Files[0] != "sample.md" {
			t.Errorf("%s: the file was read, so it is listed: %v", name, inc.Files)
		}
		if len(inc.Warnings) != 1 || !strings.Contains(inc.Warnings[0], "sample.md") || !strings.Contains(inc.Warnings[0], "~~~") || !strings.Contains(inc.Warnings[0], "indent") {
			t.Errorf("%s: want one warning naming the file and the fix, got %v", name, inc.Warnings)
		}
	}
}

func TestAFormatSpellsItsOwnDirectives(t *testing.T) {
	plain := Format{Name: "plain", TextDirective: "@include"}
	fsys := files(map[string]string{"a.txt": "A\n", "b.txt": "B\n"})
	out, inc, err := Includes("Top.\n\n@include a.txt\n\n@include-code-block b.txt\n", fsys, plain)
	if err != nil {
		t.Fatal(err)
	}
	if out != "Top.\n\nA\n\n@include-code-block b.txt\n" || len(inc.Files) != 1 {
		t.Errorf("got %q %v", out, inc.Files)
	}
}

func TestDirectivesMustStartTheirLine(t *testing.T) {
	fsys := files(map[string]string{"a.txt": "A\n"})
	for _, src := range []string{"see @include-text-block a.txt here\n", "    @include-text-block a.txt\n", "> @include-text-block a.txt\n", "`@include-text-block a.txt`\n"} {
		if out, _, _ := Includes(src, fsys, Markdown); out != src {
			t.Errorf("%q was changed to %q", src, out)
		}
	}
}

func TestBlankLinesBeforeADirectiveAtTheStartDoNotBecomeALeadingBlankLine(t *testing.T) {
	fsys := files(map[string]string{"a.txt": "A\n"})
	for _, src := range []string{"\n@include-text-block a.txt\n\nAfter.\n", "\n\n\n@include-text-block a.txt\n\nAfter.\n"} {
		out, _, _ := Includes(src, fsys, Markdown)
		if out != "A\n\nAfter.\n" {
			t.Errorf("%q gave %q", src, out)
		}
	}
	// after other text the blank-line rule is unchanged
	if out, _, _ := Includes("Top.\n@include-text-block a.txt\n", fsys, Markdown); out != "Top.\n\nA\n" {
		t.Errorf("got %q", out)
	}
}
