// Copyright (C) 2017-2026 R. S. Doiel
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package frontmatter

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// These tests run release.bash (workspace DR-0012 and DR-0013) in a scratch repository
// with a bare origin and a stand-in for the gh command. The stand-in serves
// the release JSON it finds in $FAKE_GH_VIEW (exit 1 when there is none) and
// appends every call to $FAKE_GH_LOG, so nothing reaches GitHub.

const fakeGh = `#!/bin/sh
echo "$*" >> "$FAKE_GH_LOG"
case "$1 $2" in
"release view")
	if [ -f "$FAKE_GH_VIEW" ]; then cat "$FAKE_GH_VIEW"; exit 0; fi
	exit 1 ;;
esac
exit 0
`

type releaseRepo struct {
	dir, log, view, head string
	t                    *testing.T
}

func sha(b []byte) string {
	h := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(h[:])
}

// newReleaseRepo makes a pushed repository holding release.bash, the Makefile
// (for check-committed), a codemeta.json for v9.9.9 and a dist/ with two zips.
func newReleaseRepo(t *testing.T) *releaseRepo {
	t.Helper()
	for _, tool := range []string{"git", "make", "jq", "bash", "sha256sum"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not available", tool)
		}
	}
	root := t.TempDir()
	r := &releaseRepo{dir: filepath.Join(root, "x"), log: filepath.Join(root, "gh.log"), view: filepath.Join(root, "view.json"), t: t}
	origin := filepath.Join(root, "origin.git")
	bin := filepath.Join(root, "bin")
	for _, d := range []string{bin, filepath.Join(r.dir, "dist")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	r.git(root, "init", "-q", "--bare", origin)
	r.git(root, "init", "-q", "-b", "main", r.dir)
	r.git(r.dir, "remote", "add", "origin", origin)
	for _, f := range []string{"release.bash", "Makefile"} {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		r.write(f, string(b), 0o755)
	}
	r.write("codemeta.json", `{"version": "9.9.9", "releaseNotes": "Notes."}`, 0o644)
	r.write(".gitignore", "dist/\n", 0o644)
	r.git(r.dir, "add", "-A")
	r.git(r.dir, "commit", "-q", "-m", "one")
	r.git(r.dir, "push", "-q", "origin", "main")
	r.head = strings.TrimSpace(r.git(r.dir, "rev-parse", "HEAD"))
	for name, body := range map[string]string{"x-v9.9.9-Linux.zip": "linux", "x-v9.9.9-macOS.zip": "macos"} {
		r.write(filepath.Join("dist", name), body, 0o644)
	}
	if err := os.WriteFile(filepath.Join(bin, "gh"), []byte(fakeGh), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_GH_LOG", r.log)
	t.Setenv("FAKE_GH_VIEW", r.view)
	return r
}

func (r *releaseRepo) write(name, body string, mode os.FileMode) {
	r.t.Helper()
	if err := os.WriteFile(filepath.Join(r.dir, name), []byte(body), mode); err != nil {
		r.t.Fatal(err)
	}
}

func (r *releaseRepo) git(dir string, args ...string) string {
	r.t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	c.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.org", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.org")
	out, err := c.CombinedOutput()
	if err != nil {
		r.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

// asset is one entry of the stand-in's `gh release view` answer.
type asset struct{ name, body, state string }

// draft writes the stand-in's release JSON: a release for v9.9.9 that is a
// draft or not, aimed at target, holding assets whose digest is the digest of body.
func (r *releaseRepo) draft(isDraft bool, target string, assets ...asset) {
	var list []string
	for _, a := range assets {
		state := a.state
		if state == "" {
			state = "uploaded"
		}
		list = append(list, fmt.Sprintf(`{"name":%q,"size":%d,"state":%q,"digest":%q}`, a.name, len(a.body), state, sha([]byte(a.body))))
	}
	j := fmt.Sprintf(`{"isDraft":%t,"tagName":"v9.9.9","targetCommitish":%q,"assets":[%s]}`, isDraft, target, strings.Join(list, ","))
	if err := os.WriteFile(r.view, []byte(j), 0o644); err != nil {
		r.t.Fatal(err)
	}
}

// release runs release.bash with answer on its input and returns the combined
// output, the exit code and the lines of the gh log.
func (r *releaseRepo) release(answer string, args ...string) (string, int, []string) {
	r.t.Helper()
	os.Remove(r.log)
	c := exec.Command("bash", append([]string{"release.bash"}, args...)...)
	c.Dir = r.dir
	c.Stdin = strings.NewReader(answer)
	out, err := c.CombinedOutput()
	code := 0
	if err != nil {
		ee, ok := err.(*exec.ExitError)
		if !ok {
			r.t.Fatal(err)
		}
		code = ee.ExitCode()
	}
	var calls []string
	if b, err := os.ReadFile(r.log); err == nil {
		calls = strings.Split(strings.TrimSpace(string(b)), "\n")
	}
	return string(out), code, calls
}

// uploads picks the `release upload` calls out of the gh log.
func uploads(calls []string) (names []string, clobbered map[string]bool) {
	clobbered = map[string]bool{}
	for _, c := range calls {
		if !strings.HasPrefix(c, "release upload ") {
			continue
		}
		f := strings.Fields(c)
		name := filepath.Base(f[3])
		names = append(names, name)
		if strings.Contains(c, "--clobber") {
			clobbered[name] = true
		}
	}
	return names, clobbered
}

func created(calls []string) bool {
	for _, c := range calls {
		if strings.HasPrefix(c, "release create ") {
			return true
		}
	}
	return false
}

/** TestReleaseNewDraft: a module release is a draft with notes and no files. It is
 * created at HEAD, nothing is uploaded, and no commit is made (DR-0012: the old
 * script ran make save and made a "prep" commit).
 */
func TestReleaseNewDraft(t *testing.T) {
	r := newReleaseRepo(t)
	out, code, calls := r.release("y\n")
	if code != 0 || !created(calls) {
		t.Fatalf("exit %d, created %v\n%s", code, created(calls), out)
	}
	if names, _ := uploads(calls); len(names) != 0 {
		t.Errorf("a module release has nothing to upload, got %v", names)
	}
	if !strings.Contains(strings.Join(calls, "\n"), "--target "+r.head) {
		t.Errorf("the draft is not created at HEAD %s:\n%s", r.head, strings.Join(calls, "\n"))
	}
	if got := strings.TrimSpace(r.git(r.dir, "rev-parse", "HEAD")); got != r.head {
		t.Errorf("the script made a commit: HEAD was %s, is %s", r.head, got)
	}
	if _, err := os.Stat(filepath.Join(r.dir, "release_notes.tmp")); err == nil {
		t.Error("release_notes.tmp was left behind")
	}
}

/** TestReleaseExistingDraftForHead is the resume case for a release with no files:
 * the draft is already complete, so the script creates nothing, uploads nothing
 * and says to finalize it.
 */
func TestReleaseExistingDraftForHead(t *testing.T) {
	r := newReleaseRepo(t)
	r.draft(true, r.head)
	out, code, calls := r.release("y\n")
	if names, _ := uploads(calls); code != 0 || created(calls) || len(names) != 0 || !strings.Contains(out, "finalize") {
		t.Errorf("exit %d, created %v, uploads %v, output:\n%s", code, created(calls), names, out)
	}
}

/** TestReleaseRefusesWhatItMustNotResume: a published release, a draft for another
 * commit and a draft whose target is a branch name stop the script with one line
 * and create nothing.
 */
func TestReleaseRefusesWhatItMustNotResume(t *testing.T) {
	r := newReleaseRepo(t)
	for name, c := range map[string]struct {
		draft  bool
		target string
		want   string
	}{
		"published":        {false, r.head, "not a draft"},
		"other commit":     {true, strings.Repeat("a", 40), "targets"},
		"branch as target": {true, "main", "targets"},
	} {
		r.draft(c.draft, c.target)
		out, code, calls := r.release("y\n")
		if code == 0 || created(calls) || !strings.Contains(out, c.want) {
			t.Errorf("%s: exit %d, created %v, output:\n%s", name, code, created(calls), out)
		}
	}
}

/** TestReleaseDryRun says what would be created and from which commit, and changes
 * nothing.
 */
func TestReleaseDryRun(t *testing.T) {
	r := newReleaseRepo(t)
	out, code, calls := r.release("", "--dry-run")
	if code != 0 || created(calls) || !strings.Contains(out, r.head) {
		t.Errorf("exit %d, created %v, output:\n%s", code, created(calls), out)
	}
}

/** TestReleaseNeedsCommittedAndPushed: an uncommitted change, or a commit that is not
 * on origin, stops the script before it looks at GitHub.
 */
func TestReleaseNeedsCommittedAndPushed(t *testing.T) {
	r := newReleaseRepo(t)
	r.write("codemeta.json", `{"version": "9.9.9", "releaseNotes": "Changed."}`, 0o644)
	if _, code, calls := r.release("y\n"); code == 0 || created(calls) {
		t.Errorf("uncommitted change: exit %d, created %v; want a refusal", code, created(calls))
	}
	r.git(r.dir, "commit", "-q", "-am", "unpushed")
	if _, code, calls := r.release("y\n"); code == 0 || created(calls) {
		t.Errorf("unpushed commit: exit %d, created %v; want a refusal", code, created(calls))
	}
}
