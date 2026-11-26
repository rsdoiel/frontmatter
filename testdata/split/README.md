# Conformance corpus: split

Each case is three files in this directory:

- `NAME.in`: the input, byte for byte (`.gitattributes` keeps git from changing line endings here).
- `NAME.json`: `{"found": bool, "error": ""|"unclosed"|"parse"|"not_mapping", "meta": {...}}`. `found` is true when a front matter block was read.
- `NAME.out`: the expected body, byte for byte.

The body is the whole input when there is no block and when the block is unclosed (nothing was split). When the block is closed but its YAML is bad (`parse`) or is not a mapping (`not_mapping`), the delimiters were found, `found` is true, the body is what follows the closing line, and `meta` is absent. Cases marked `new` came from no existing copy: they fix a question the copies left open.

| Case | From |
|---|---|
| `basic` | antennaApp DR-0022 |
| `rule-in-body` | antennaApp DR-0022 |
| `two-rules-and-no-block` | antennaApp DR-0022 |
| `first-line-is-not-a-rule` | antennaApp DR-0022 |
| `no-final-newline` | antennaApp DR-0022 |
| `crlf-kept-in-the-body` | antennaApp DR-0022 |
| `dots-close-the-block` | antennaApp DR-0022 |
| `dots-in-the-body` | antennaApp DR-0022 |
| `dots-with-no-block` | antennaApp DR-0022 |
| `empty-block` | antennaApp DR-0022 |
| `unclosed-block` | antennaApp DR-0022; harvey |
| `closing-rule-ends-the-file` | cmarkbrowser |
| `closing-rule-then-crlf-only` | cmarkbrowser |
| `simple` | cmarkbrowser |
| `no-front-matter` | cmarkbrowser |
| `empty-file` | new |
| `only-a-rule` | new |
| `only-a-rule-no-newline` | new |
| `blank-block` | new |
| `blank-first-line` | new |
| `four-dashes-is-not-a-rule` | new |
| `rule-with-trailing-space` | new |
| `closing-rule-with-trailing-space` | new |
| `leading-blank-line-in-the-body-is-kept` | harvey |
| `record-format-strings-and-lists` | knowledge |
| `nested-table-and-list` | new |
| `multibyte-text` | new |
| `numbers-and-booleans` | new |
| `invalid-yaml` | new |
| `yaml-needs-a-colon` | antennaApp DR-0022 |
| `duplicate-key` | new |
| `list-is-not-a-mapping` | knowledge |
| `scalar-is-not-a-mapping` | new |
| `unclosed-block-crlf` | new |
| `comment-only-block` | new |
| `null-block` | new |
| `unquoted-colon-in-a-value` | found on real data: workspace skill and command files (harvey's lenient reader exists for these) |
| `bom-before-the-block` | new; author chose to skip a leading BOM, 2026-11-26 |
| `bom-and-no-block` | new; the input is returned as it was, BOM included |
| `bom-and-unclosed-block` | new |
