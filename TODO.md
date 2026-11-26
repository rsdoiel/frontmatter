
# Action Items

Design, decisions and the session plan live in the workspace under `agents/projects/frontmatter/`.

## Before v0.0.1

- [x] Front matter core, macros, includes (sessions F1 to F3).
- [ ] The author tags and pushes (session F4): `make check-release`, then `release.bash`.

## Later

- [ ] Documentation site in the Markdown-web pattern (index.md, `/.well-known/toc.opml`, `website.mak`, `page.tmpl`); the Makefile's `website`, `publish` and `clean-website` targets wait for it.
- [ ] Edit one key in place, keeping comments and order (knowledge's `setMappingField` and `spliceFrontmatter` are the model): its own decision record first.
- [ ] A Fountain `Format` for includes, when a Fountain tool needs one.
- [ ] Directives in a CRLF document: a `\r` before the line end is not accepted today, as in antennaApp.
- [ ] Adoption by cmarkbrowser, antennaApp, harvey and knowledge (plan sessions F5 to F8).
