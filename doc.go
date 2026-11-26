/** Package frontmatter works with the front matter of Markdown and other plain
 * text documents: it finds the YAML block at the top of a file, returns it,
 * parses it, strips it, expands {{page.KEY}} macros from it, and expands
 * @include-text-block and @include-code-block directives.
 *
 * Only YAML is supported. A block is the text between a first line that is
 * exactly "---" and the next line that is exactly "---" or "...". Everything
 * after the closing line is the body and is returned as the original bytes.
 * A UTF-8 byte-order mark before the opening line is skipped. A block that is
 * not closed is an error; the module applies no policy beyond reporting it, so
 * a reader may show the raw text and a validator may fail.
 *
 * Metadata values are what gopkg.in/yaml.v3 decodes, unchanged: a bare date
 * such as 2026-08-19 is a time.Time, a quoted one is a string, a number is an
 * int or a float, a list is []any and a nested table is map[string]any. Only
 * YAML that conforms to the YAML specification is read; there is no lenient mode.
 *
 * Errors are sentinels: use errors.Is(err, frontmatter.ErrUnclosed), never the
 * message text.
 *
 * Example:
 *   meta, body, err := frontmatter.Split([]byte("---\ntitle: Hi\n---\nText.\n"))
 *   // meta["title"] == "Hi", string(body) == "Text.\n", err == nil
 */
package frontmatter
