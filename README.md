
# frontmatter

A Go module for the front matter of Markdown and other plain text documents: split it from the body, read it, strip it, expand `{{page.KEY}}` macros from it and expand `@include` directives. YAML only.

It extends what the other modules and programs around the Markdown web need (cmarkbrowser, antennaApp and others), so that they agree about where front matter ends, what a macro expands to and how an include behaves.

```shell
go get github.com/rsdoiel/frontmatter
```

Requires Go 1.27.4 or later, `github.com/yuin/goldmark` (for anything that must understand Markdown) and `gopkg.in/yaml.v3`.

## Front matter

Front matter is a YAML block at the very top of a file. The first line is `---` and nothing else. The block ends at the next line that is `---` or `...` and nothing else. Everything after is the body, returned as the original bytes: line endings and the final newline are never changed. A UTF-8 byte-order mark before the first line is skipped. A later `---` line is body (a Markdown rule).

```go
meta, body, err := frontmatter.Split([]byte("---\ntitle: Hi\n---\nText.\n"))
// meta["title"] == "Hi", string(body) == "Text.\n", err == nil

body = frontmatter.Strip(src)        // the body, or src itself when there is no block
meta, err = frontmatter.Meta(src)    // only the metadata
raw, bodyAt, found, err := frontmatter.Block(src) // only the delimiters; no YAML is read
```

- Only YAML that conforms to the YAML specification is read. There is no lenient mode, so tools such as `yq` work on any front matter this module reads. TOML (`+++`) and JSON front matter are not recognised.
- Values are what `gopkg.in/yaml.v3` decodes: a bare date such as `2026-08-19` is a `time.Time`, a quoted one is a string.
- A block that is not closed is an error, `ErrUnclosed`. Bad YAML is `ErrParse`; YAML that is a list or a single value is `ErrNotMapping`. Test with `errors.Is`. The module applies no policy beyond reporting: a reader may show the raw text, a validator may fail. When the block is closed but unusable, `Split` still returns the body.
- A document with no block, or an empty one, has an empty (not nil) metadata map.

## Macros

`{{page.KEY}}` in the text is replaced by the front matter value KEY, so a page can start with `# {{page.title}}`. Spaces inside the braces are allowed, and a dotted key reaches into a table (`{{page.source.title}}`). A list shows its items joined with commas; a date shows as `2026-11-19`.

```go
out, report := frontmatter.Expand("# {{page.title}}", map[string]any{"title": "Hi"})
// out == "# Hi"; report.Missing lists keys that had no value
```

`Expand` takes Markdown and returns Markdown. For a program that renders HTML, `MacroOption` is a goldmark parser option that expands the macros in the parsed document:

```go
var report frontmatter.Report
md := goldmark.New(goldmark.WithParserOptions(frontmatter.MacroOption(meta, &report)))
```

Both leave code spans and code blocks alone, make one pass (a value that holds a macro is shown as written), and insert a value so that it cannot become markup, a link, HTML or the start of a heading or list. A key that is absent, null or a table leaves the macro visible and is reported once in `Report.Missing`. When the page has a `link` value, the expanded title is a link to it (http, https, mailto or relative addresses only), unless the macro is already inside a link.

## Includes

```go
out, included, err := frontmatter.Includes(text, os.DirFS("."), frontmatter.Markdown)
```

In the Markdown format, `@include-text-block FILE` at the start of a line is replaced by the text of FILE, and `@include-code-block FILE [LANGUAGE]` by FILE as a code block:

~~~
@include-text-block author.md
@include-code-block main.go go
~~~

- Files are read through the `fs.FS` you pass, so the root is yours to choose. A name that is absolute or leaves the root with `..` is refused.
- A text file is itself expanded; a code file never is. An included file's front matter is dropped.
- A code file is written as `~~~LANGUAGE` ... `~~~`. A file with a line that would close that fence early (nothing but three or more tildes) is not included: the directive is left as written and a warning says to indent the example in that file.
- A directive inside a fenced code block is left alone, so a page can show the syntax.
- A missing file, a directory or an unreadable file leaves the directive as written. Everything is reported in `included.Warnings`; `included.Files` lists every file a directive named, once each, in the order met.
- Nesting deeper than three levels draws a warning; deeper than five (`ErrTooDeep`), or a file that includes itself (`ErrCycle`), is an error that names the chain.

A `Format` says how another kind of document spells its directives (for example a Fountain format with no code includes). The engine knows nothing else about the format.

## Conformance corpus

`testdata/split/` holds one input, one expected body and one expected result per case, byte for byte, with a README that says where each case came from. It is the specification of the front matter rules and can be used to test a program that has its own splitter.

## Status

Version 0.0.1, unreleased work in progress. See `CHANGES.md` and `TODO.md`.

### Authors

- Doiel, R. S.

## Related resources

- [Getting Help, Reporting bugs](https://github.com/rsdoiel/frontmatter/issues)
- [LICENSE](https://www.gnu.org/licenses/agpl-3.0.txt)
- [About](about.md)
