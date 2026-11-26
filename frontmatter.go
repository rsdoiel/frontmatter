package frontmatter

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"
)

// utf8BOM is the byte-order mark some editors write at the start of a UTF-8 file.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

/** Block finds the front matter block at the top of src and says where the body
 * starts. It reads no YAML: only the delimiters. A UTF-8 byte-order mark before
 * the opening "---" is skipped; when there is no block, src is returned as it
 * is, mark included.
 *
 * Parameters:
 *   src ([]byte) — the whole document
 *
 * Returns:
 *   []byte — the text between the delimiter lines (lines joined with "\n"), nil when there is no block
 *   int — the offset in src where the body starts; 0 when there is no block
 *   bool — true when a closed block was found
 *   error — wraps ErrUnclosed when the first line is "---" and nothing closes it; nil otherwise
 *
 * Example:
 *   raw, bodyAt, found, err := frontmatter.Block([]byte("---\ntitle: T\n---\nBody.\n"))
 *   // string(raw) == "title: T", bodyAt == 17, found, err == nil
 */
func Block(src []byte) (raw []byte, bodyAt int, found bool, err error) {
	var lines [][]byte
	pos, first := 0, true
	if bytes.HasPrefix(src, utf8BOM) {
		pos = len(utf8BOM) // a byte-order mark is not part of the first line
	}
	for pos < len(src) {
		end := len(src)
		if nl := bytes.IndexByte(src[pos:], '\n'); nl >= 0 {
			end = pos + nl
		}
		line := bytes.TrimSuffix(src[pos:end], []byte("\r"))
		next := min(end+1, len(src))
		if first {
			if !bytes.Equal(line, []byte("---")) {
				return nil, 0, false, nil
			}
			first = false
		} else if bytes.Equal(line, []byte("---")) || bytes.Equal(line, []byte("...")) {
			return bytes.Join(lines, []byte("\n")), next, true, nil
		} else {
			lines = append(lines, line)
		}
		pos = next
	}
	if first {
		return nil, 0, false, nil
	}
	return nil, 0, false, ErrUnclosed
}

/** Split returns the parsed metadata and the body.
 *
 * Parameters:
 *   src ([]byte) — the whole document
 *
 * Returns:
 *   map[string]any — the metadata; empty and not nil when there is no block or the block is empty; nil on error
 *   []byte — the body: the original bytes after the closing line, src itself when there is no block or the block is unclosed
 *   error — wraps ErrUnclosed, ErrParse or ErrNotMapping; the body is still returned for the last two
 *
 * Example:
 *   meta, body, err := frontmatter.Split([]byte("---\ntitle: Hi\n---\nText.\n"))
 */
func Split(src []byte) (meta map[string]any, body []byte, err error) {
	raw, bodyAt, found, err := Block(src)
	if err != nil {
		return nil, src, err
	}
	if !found {
		return map[string]any{}, src, nil
	}
	meta, err = parseBlock(raw)
	return meta, src[bodyAt:], err
}

/** Strip returns src without its front matter. It needs only the delimiters, so
 * a block with bad YAML is stripped too; a document with no block, or an
 * unclosed one, is returned unchanged.
 *
 * Parameters:
 *   src ([]byte) — the whole document
 *
 * Returns:
 *   []byte — the body
 *
 * Example:
 *   body := frontmatter.Strip([]byte("---\na: b\n---\nBody\n")) // "Body\n"
 */
func Strip(src []byte) []byte {
	_, bodyAt, found, err := Block(src)
	if err != nil || !found {
		return src
	}
	return src[bodyAt:]
}

/** Meta returns only the metadata of src.
 *
 * Parameters:
 *   src ([]byte) — the whole document
 *
 * Returns:
 *   map[string]any — the metadata; empty and not nil when there is none; nil on error
 *   error — wraps ErrUnclosed, ErrParse or ErrNotMapping
 *
 * Example:
 *   meta, err := frontmatter.Meta([]byte("---\ntitle: Hi\n---\n"))
 */
func Meta(src []byte) (map[string]any, error) {
	meta, _, err := Split(src)
	return meta, err
}

// parseBlock parses the text of a closed block as a YAML mapping. An empty
// block, or one holding only comments or a null, is no metadata.
func parseBlock(raw []byte) (map[string]any, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrParse, err)
	}
	if len(doc.Content) == 0 {
		return map[string]any{}, nil
	}
	top := doc.Content[0]
	if top.Kind == yaml.ScalarNode && top.Tag == "!!null" {
		return map[string]any{}, nil
	}
	if top.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%w: the block is a %s", ErrNotMapping, kindName(top.Kind))
	}
	meta := map[string]any{}
	if err := top.Decode(&meta); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrParse, err)
	}
	return meta, nil
}

// kindName names a YAML node kind for an error message.
func kindName(k yaml.Kind) string {
	switch k {
	case yaml.SequenceNode:
		return "list"
	case yaml.ScalarNode:
		return "single value"
	case yaml.AliasNode:
		return "alias"
	}
	return "value"
}
