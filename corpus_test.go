package frontmatter

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// corpusCase is one case of testdata/split: see that directory's README.md.
type corpusCase struct {
	name  string
	in    []byte
	out   []byte
	Found bool           `json:"found"`
	Error string         `json:"error"`
	Meta  map[string]any `json:"meta"`
}

// loadCorpus reads every case of testdata/split.
func loadCorpus(t *testing.T) []corpusCase {
	t.Helper()
	ins, err := filepath.Glob(filepath.Join("testdata", "split", "*.in"))
	if err != nil || len(ins) == 0 {
		t.Fatalf("no corpus found in testdata/split: %v", err)
	}
	var cases []corpusCase
	for _, in := range ins {
		base := strings.TrimSuffix(in, ".in")
		var c corpusCase
		c.name = filepath.Base(base)
		if c.in, err = os.ReadFile(in); err != nil {
			t.Fatal(err)
		}
		if c.out, err = os.ReadFile(base + ".out"); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		j, err := os.ReadFile(base + ".json")
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if err := json.Unmarshal(j, &c); err != nil {
			t.Fatalf("%s.json: %v", c.name, err)
		}
		cases = append(cases, c)
	}
	return cases
}

// wantErr maps a case's error word to its sentinel (nil for none).
func wantErr(t *testing.T, word string) error {
	t.Helper()
	switch word {
	case "":
		return nil
	case "unclosed":
		return ErrUnclosed
	case "parse":
		return ErrParse
	case "not_mapping":
		return ErrNotMapping
	}
	t.Fatalf("unknown error word %q in the corpus", word)
	return nil
}

// sameErr reports whether got is want (nil) or wraps it.
func sameErr(got, want error) bool {
	if want == nil {
		return got == nil
	}
	return errors.Is(got, want)
}

// normalize turns a metadata map into what encoding/json makes of it, so values
// from YAML and from the case's JSON compare equal.
func normalize(t *testing.T, m map[string]any) any {
	t.Helper()
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("metadata is not JSON: %v", err)
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestCorpusBlock(t *testing.T) {
	for _, c := range loadCorpus(t) {
		t.Run(c.name, func(t *testing.T) {
			raw, bodyAt, found, err := Block(c.in)
			// Block reads only delimiters, so only an unclosed block is its error.
			want := error(nil)
			if c.Error == "unclosed" {
				want = ErrUnclosed
			}
			if !sameErr(err, want) {
				t.Fatalf("error = %v, want %v", err, want)
			}
			if found != c.Found {
				t.Fatalf("found = %v, want %v", found, c.Found)
			}
			if !found {
				if bodyAt != 0 || raw != nil {
					t.Fatalf("no block: bodyAt = %d, raw = %q, want 0 and nil", bodyAt, raw)
				}
				return
			}
			if bodyAt < 0 || bodyAt > len(c.in) || !bytes.Equal(c.in[bodyAt:], c.out) {
				t.Fatalf("src[bodyAt:] = %q, want %q", c.in[min(max(bodyAt, 0), len(c.in)):], c.out)
			}
			if !bytes.HasPrefix(bytes.TrimPrefix(c.in, []byte{0xEF, 0xBB, 0xBF}), []byte("---")) {
				t.Fatalf("found a block in a document that does not start with a rule")
			}
		})
	}
}

func TestCorpusSplit(t *testing.T) {
	for _, c := range loadCorpus(t) {
		t.Run(c.name, func(t *testing.T) {
			meta, body, err := Split(c.in)
			if want := wantErr(t, c.Error); !sameErr(err, want) {
				t.Fatalf("error = %v, want %v", err, want)
			}
			if !bytes.Equal(body, c.out) {
				t.Fatalf("body = %q, want %q", body, c.out)
			}
			if c.Error != "" {
				if meta != nil {
					t.Fatalf("metadata = %v on an error, want nil", meta)
				}
				return
			}
			if meta == nil {
				t.Fatalf("metadata is nil, want an empty map at least")
			}
			if got, want := normalize(t, meta), normalize(t, c.Meta); !reflect.DeepEqual(got, want) {
				t.Fatalf("metadata = %v, want %v", got, want)
			}
		})
	}
}

func TestCorpusStrip(t *testing.T) {
	for _, c := range loadCorpus(t) {
		t.Run(c.name, func(t *testing.T) {
			if got := Strip(c.in); !bytes.Equal(got, c.out) {
				t.Fatalf("Strip = %q, want %q", got, c.out)
			}
		})
	}
}

func TestCorpusMeta(t *testing.T) {
	for _, c := range loadCorpus(t) {
		t.Run(c.name, func(t *testing.T) {
			meta, err := Meta(c.in)
			if want := wantErr(t, c.Error); !sameErr(err, want) {
				t.Fatalf("error = %v, want %v", err, want)
			}
			if c.Error != "" {
				if meta != nil {
					t.Fatalf("metadata = %v on an error, want nil", meta)
				}
				return
			}
			if meta == nil {
				t.Fatalf("metadata is nil, want an empty map at least")
			}
			if got, want := normalize(t, meta), normalize(t, c.Meta); !reflect.DeepEqual(got, want) {
				t.Fatalf("metadata = %v, want %v", got, want)
			}
		})
	}
}
