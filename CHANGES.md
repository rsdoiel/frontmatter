
# Change log

## 0.0.1 (2026-11-26)

First release.

- `Split`, `Block`, `Strip` and `Meta` read the YAML front matter block at the top of a file. The block is closed by `---` or `...`; a UTF-8 byte-order mark before it is skipped; an unclosed block is an error (`ErrUnclosed`); the body is the original bytes. Only YAML that conforms to the YAML specification is read.
- `Expand` and `MacroOption` expand `{{page.KEY}}` macros, for Markdown to Markdown and for a goldmark parser. Code is never touched; values cannot become markup; the title becomes a link when the page has a safe `link` value.
- `Includes` expands `@include-text-block` and `@include-code-block` through an `fs.FS`, with nesting, cycle and depth limits, a report of the files named, directives in fenced code left alone, and a code file fenced with `~~~` and refused when one of its lines would close the fence. `Format` lets another kind of document spell its own directives.
- A conformance corpus in `testdata/split`.
