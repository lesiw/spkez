package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"lesiw.io/cmdio"
)

func init() {
	cmdio.Trace = io.Discard
}

func TestLs(t *testing.T) {
	var out bytes.Buffer
	swap[io.Writer](t, &stdout, &out)
	dir := t.TempDir()
	writeKey(t, dir, "foo", "foo value")
	writeKey(t, dir, "bar", "bar value")
	writeKey(t, dir, "biz/baz", "baz value")

	err := ls(dir)

	if err != nil {
		t.Errorf("ls(%v) = %q, want <nil>", dir, err)
	}
	outexp := `bar
biz/baz
foo
`
	if got, want := out.String(), outexp; got != want {
		t.Errorf("ls(%v) out -want +got\n%s", dir, cmp.Diff(got, want))
	}
}

// TestGetSet tests that what spkez writes can be read with the same password.
func TestGetSet(t *testing.T) {
	c := new(echocdr)
	swap(t, &rnr, new(cmdio.Runner).WithCommander(c))
	dir := t.TempDir()
	t.Setenv("SPKEZPASS", "test passphrase")

	seterr := set(dir, "hello", "hello world")
	value, geterr := get(dir, "hello")

	if seterr != nil {
		t.Errorf("set(%v, %q, %q) = %q, want <nil>",
			dir, "hello", "hello world", seterr)
	} else if readKey(t, dir, "hello") == "hello world" {
		t.Errorf("hello key file = %q, want encrypted value", "hello world")
	}
	if geterr != nil {
		t.Errorf("get(%v, %q) = _, %q; want _, <nil>", dir, "hello", geterr)
	} else if got, want := value, "hello world"; got != want {
		t.Errorf("get(%v, %q) = %q, <nil>; want %q, <nil>",
			dir, "hello", got, want)
	}
	cmdexp := [][]string{
		{"git", "-C", dir, "add", "hello"},
		{"git", "-C", dir, "status", "-s"},
		{"git", "-C", dir, "commit", "-am", `set "hello"`},
		{"git", "-C", dir, "push"},
	}
	if got, want := c.cmd, cmdexp; !cmp.Equal(got, want) {
		t.Errorf("cmd -want +got\n%s", cmp.Diff(got, want))
	}
}

// TestGetSetFail tests that what spkez writes cannot be read with a different
// password.
func TestGetSetFail(t *testing.T) {
	c := new(echocdr)
	swap(t, &rnr, new(cmdio.Runner).WithCommander(c))
	dir := t.TempDir()
	t.Setenv("SPKEZPASS", "test passphrase")

	seterr := set(dir, "hello", "hello world")
	t.Setenv("SPKEZPASS", "new passphrase")
	value, geterr := get(dir, "hello")

	if seterr != nil {
		t.Errorf("set(%v, %q, %q) = %q, want <nil>",
			dir, "hello", "hello world", seterr)
	} else if readKey(t, dir, "hello") == "hello world" {
		t.Errorf("hello key file = %q, want encrypted value", "hello world")
	}
	werr := `failed to decrypt "hello": cipher: message authentication failed`
	if geterr == nil {
		t.Errorf("get(%v, %q) = _, <nil>; want err", dir, "hello")
	} else if got, want := geterr.Error(), werr; got != want {
		t.Errorf("get(%v, %q) = _, %q; want _, %q", dir, "hello", got, want)
	}
	if value == "hello world" {
		t.Errorf("get(%v, %q) = %q, _; want %q",
			dir, "hello", "hello world", "")
	}
	cmdexp := [][]string{
		{"git", "-C", dir, "add", "hello"},
		{"git", "-C", dir, "status", "-s"},
		{"git", "-C", dir, "commit", "-am", `set "hello"`},
		{"git", "-C", dir, "push"},
	}
	if got, want := c.cmd, cmdexp; !cmp.Equal(got, want) {
		t.Errorf("cmd -want +got\n%s", cmp.Diff(got, want))
	}
}

func TestDel(t *testing.T) {
	c := new(echocdr)
	swap(t, &rnr, new(cmdio.Runner).WithCommander(c))
	dir := t.TempDir()
	writeKey(t, dir, "foo", "foo value")
	writeKey(t, dir, "bar", "bar value")
	writeKey(t, dir, "biz/baz", "baz value")

	err := del(dir, "biz/baz")

	if err != nil {
		t.Errorf("del(%q, %q) = %q, want <nil>", dir, "bar", err)
	}
	if k := "foo"; !fileExists(t, dir, k) {
		t.Errorf("missing key: %q", k)
	}
	if k := "bar"; !fileExists(t, dir, k) {
		t.Errorf("missing key: %q", k)
	}
	if k := "biz/baz"; fileExists(t, dir, "biz/baz") {
		t.Errorf("%q key present, expected it to be deleted", k)
	}
	cmdexp := [][]string{
		{"git", "-C", dir, "add", "biz/baz"},
		{"git", "-C", dir, "status", "-s"},
		{"git", "-C", dir, "commit", "-am", `del "biz/baz"`},
		{"git", "-C", dir, "push"},
	}
	if got, want := c.cmd, cmdexp; !cmp.Equal(got, want) {
		t.Errorf("cmd -want +got\n%s", cmp.Diff(got, want))
	}
}

type echocdr struct {
	cmd [][]string
	cnt map[string]int
}

func (c *echocdr) Command(
	_ context.Context, env map[string]string, args ...string,
) cmdio.Command {
	c.cmd = append(c.cmd, args)
	s := fmt.Sprintf("%v", args)
	if c.cnt == nil {
		c.cnt = make(map[string]int)
	}
	c.cnt[s]++
	if c.cnt[s] > 1 {
		s = fmt.Sprintf("%s[%d]", s, c.cnt[s])
	}
	return rwcmd{bytes.NewBufferString(s + "\n")}
}

type rwcmd struct{ io.ReadWriter }

func (rwcmd) Close() error   { return nil }
func (rwcmd) String() string { return "<nop>" }
func (rwcmd) Attach() error  { return nil }
func (rwcmd) Code() int      { return 0 }
func (rwcmd) Log(io.Writer)  {}

func swap[T any](t *testing.T, orig *T, with T) {
	t.Helper()
	o := *orig
	t.Cleanup(func() { *orig = o })
	*orig = with
}

func writeKey(t *testing.T, dir, key, value string) {
	t.Helper()
	path := filepath.Join(dir, key)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if _, err := file.WriteString(value); err != nil {
		t.Fatal(err)
	}
}

func readKey(t *testing.T, dir, key string) string {
	t.Helper()
	path := filepath.Join(dir, key)
	buf, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(buf)
}

func fileExists(t *testing.T, dir, key string) bool {
	t.Helper()
	path := filepath.Join(dir, key)
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}
