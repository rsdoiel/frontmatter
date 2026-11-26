package frontmatter

import (
	"bytes"
	"errors"
	"testing"
	"time"
)

func TestBlockReturnsTheRawTextWithoutCarriageReturns(t *testing.T) {
	for _, tc := range []struct{ name, src, raw string }{
		{"lf", "---\ntitle: T\nauthor: A\n---\nBody\n", "title: T\nauthor: A"},
		{"crlf", "---\r\ntitle: T\r\nauthor: A\r\n---\r\nBody\r\n", "title: T\nauthor: A"},
		{"empty", "---\n---\nBody\n", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, _, found, err := Block([]byte(tc.src))
			if err != nil || !found {
				t.Fatalf("found = %v, err = %v", found, err)
			}
			if string(raw) != tc.raw {
				t.Fatalf("raw = %q, want %q", raw, tc.raw)
			}
		})
	}
}

func TestErrorsNameWhatWentWrong(t *testing.T) {
	_, err := Meta([]byte("---\n- a\n---\n"))
	if !errors.Is(err, ErrNotMapping) || errors.Is(err, ErrParse) {
		t.Fatalf("a list is ErrNotMapping and not ErrParse, got %v", err)
	}
	_, err = Meta([]byte("---\ntitle: A\ntitle: B\n---\n"))
	if !errors.Is(err, ErrParse) {
		t.Fatalf("a duplicate key is ErrParse, got %v", err)
	}
}

func TestSplitReturnsTheInputItselfWhenThereIsNoBlock(t *testing.T) {
	src := []byte("# Title\n\nText.\n")
	_, body, err := Split(src)
	if err != nil || !bytes.Equal(body, src) {
		t.Fatalf("body = %q, err = %v", body, err)
	}
}

// FuzzSplit checks, for any input, that nothing panics and that the functions
// agree about where the body starts.
func FuzzSplit(f *testing.F) {
	for _, s := range []string{"", "---", "---\n", "---\n---\n", "---\na: b\n---\nx", "---\r\na: b\r\n...\r\nx", "x\n---\na: b\n---\n", "---\n- a\n---\n", "---\na: [\n---\n"} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, src []byte) {
		raw, bodyAt, found, err := Block(src)
		meta, body, serr := Split(src)
		strip := Strip(src)
		if err != nil {
			if !errors.Is(err, ErrUnclosed) || found {
				t.Fatalf("Block: err = %v, found = %v", err, found)
			}
			if !errors.Is(serr, ErrUnclosed) || !bytes.Equal(body, src) || !bytes.Equal(strip, src) {
				t.Fatalf("an unclosed block must give the input back: %v", serr)
			}
			return
		}
		want := src
		if found {
			if bodyAt < 0 || bodyAt > len(src) {
				t.Fatalf("bodyAt = %d for %d bytes", bodyAt, len(src))
			}
			want = src[bodyAt:]
		} else if raw != nil || bodyAt != 0 {
			t.Fatalf("no block but raw = %q, bodyAt = %d", raw, bodyAt)
		}
		if !bytes.Equal(body, want) || !bytes.Equal(strip, want) {
			t.Fatalf("body = %q, strip = %q, want %q", body, strip, want)
		}
		if serr == nil && meta == nil {
			t.Fatalf("no error and no metadata")
		}
	})
}

func TestValuesAreWhatYAMLv3Decodes(t *testing.T) {
	meta, err := Meta([]byte("---\nbare: 2026-08-19\nquoted: \"2026-08-19\"\nn: 3\nf: 1.5\nlist: [a, b]\ntable:\n  k: v\n---\n"))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := meta["bare"].(time.Time); !ok {
		t.Errorf("a bare date is %T, want time.Time", meta["bare"])
	}
	if _, ok := meta["quoted"].(string); !ok {
		t.Errorf("a quoted date is %T, want string", meta["quoted"])
	}
	if _, ok := meta["n"].(int); !ok {
		t.Errorf("3 is %T, want int", meta["n"])
	}
	if _, ok := meta["f"].(float64); !ok {
		t.Errorf("1.5 is %T, want float64", meta["f"])
	}
	if _, ok := meta["list"].([]any); !ok {
		t.Errorf("a list is %T, want []any", meta["list"])
	}
	if _, ok := meta["table"].(map[string]any); !ok {
		t.Errorf("a table is %T, want map[string]any", meta["table"])
	}
}
