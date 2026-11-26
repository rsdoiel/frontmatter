---
title: frontmatter
abstract: |-
  A Go module for the front matter of Markdown and other plain text documents: split it from the body, read it, strip it, expand `{{page.KEY}}` macros from it and expand `@include` directives. YAML only.
authors:
  - family_name: Doiel
    given_name: R. S.
    id: https://orcid.org/0000-0003-0900-6903


maintainer:
  - family_name: Doiel
    given_name: R. S.
    id: https://orcid.org/0000-0003-0900-6903

repository_code: https://github.com/rsdoiel/frontmatter
version: 0.0.1
license_url: https://www.gnu.org/licenses/agpl-3.0.txt

programming_language:
  - Go >= 1.27.4

keywords:
  - front matter
  - yaml
  - markdown
  - commonmark
  - fountain
  - plain text

date_released: 2026-11-26
---

About this software
===================

## frontmatter 0.0.1

- first release
- Split, Block, Strip and Meta: the YAML front matter block at the top of a file, closed by `---` or `...`, a UTF-8 byte-order mark skipped, an unclosed block an error (ErrUnclosed), the body returned as the original bytes
- Expand and MacroOption: `{{page.KEY}}` macros from the front matter, for Markdown to Markdown and for a goldmark parser; code is never touched; the title becomes a link when the page has a safe link value
- Includes: `@include-text-block` and `@include-code-block` over an fs.FS, nested text includes, cycle and depth errors, directives in fenced code left alone, a code file fenced with `~~~` and refused when a line would close the fence early; a Format says how another kind of document spells its directives
- a conformance corpus in testdata/split
- requires github.com/yuin/goldmark and gopkg.in/yaml.v3
- YAML only: TOML and JSON front matter are not read

## Authors

- [R. S. Doiel](https://orcid.org/0000-0003-0900-6903)




## Maintainers

- [R. S. Doiel](https://orcid.org/0000-0003-0900-6903)


A Go module for the front matter of Markdown and other plain text documents: split it from the body, read it, strip it, expand `{{page.KEY}}` macros from it and expand `@include` directives. YAML only.

- [License](https://www.gnu.org/licenses/agpl-3.0.txt)
- [Code Repository](https://github.com/rsdoiel/frontmatter)
  - [Issue Tracker](https://github.com/rsdoiel/frontmatter/issues)

## Programming languages

- Go >= 1.27.4




## Software Requirements

- Go >= 1.27.4
- github.com/yuin/goldmark >= 1.8.7
- gopkg.in/yaml.v3 >= 3.0.1


## Software Suggestions

- CMTools >= 0.0.46
- GNU Make >= 3


