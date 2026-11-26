package frontmatter

import "errors"

/** ErrUnclosed is wrapped by the error returned when a document's first line
 * is "---" and no closing "---" or "..." line follows.
 *
 * Example:
 *   _, _, err := frontmatter.Split([]byte("---\ntitle: T\n"))
 *   errors.Is(err, frontmatter.ErrUnclosed) // true
 */
var ErrUnclosed = errors.New("front matter is not closed")

/** ErrParse is wrapped by the error returned when the block is closed but is
 * not valid YAML (including a key that appears twice).
 *
 * Example:
 *   _, err := frontmatter.Meta([]byte("---\ntitle: [\n---\n"))
 *   errors.Is(err, frontmatter.ErrParse) // true
 */
var ErrParse = errors.New("front matter is not valid YAML")

/** ErrNotMapping is wrapped by the error returned when the block is valid YAML
 * but is a list or a single value and not a mapping of keys to values.
 *
 * Example:
 *   _, err := frontmatter.Meta([]byte("---\n- a\n---\n"))
 *   errors.Is(err, frontmatter.ErrNotMapping) // true
 */
var ErrNotMapping = errors.New("front matter is not a mapping")
