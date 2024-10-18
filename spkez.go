package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/pbkdf2"
	"lesiw.io/cmdio"
	"lesiw.io/cmdio/sys"
	repo "lesiw.io/repo/lib"
)

var (
	usage = `spkez - simple secret manager

  spkez ls                 List keys.
  spkez get [key]          Get value of [key].
  spkez del [key]          Delete [key].
  spkez set [key] [value]  Set [key] to [value].

environment variables

  SPKEZREPO  URL of the git secret store.
  SPKEZPASS  Password for encrypting and decrypting secrets.`
	errUsage = errors.New(usage)
	rnr      = sys.Runner()
	verbs    = boolmap("get", "set", "del", "ls")

	stdout io.Writer = os.Stdout
	stderr io.Writer = os.Stderr

	//go:embed version.txt
	versionfile string
	version     = strings.TrimRight(versionfile, "\n")
)

const (
	keysz  = 32
	saltsz = 16
)

func main() {
	cmdio.Trace = io.Discard
	if err := run(os.Args); err != nil {
		fmt.Fprintln(stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(os.Args) > 1 {
		if os.Args[1] == "-V" || os.Args[1] == "--version" {
			fmt.Fprintln(stdout, version)
			return nil
		} else if os.Args[1] == "-h" || os.Args[1] == "--help" {
			fmt.Fprintln(stderr, usage)
			return nil
		}
	}
	verb, err := validate(args)
	if err != nil {
		return err
	}
	dir, err := repodir(os.Getenv("SPKEZREPO"))
	if err != nil {
		return err
	}
	switch verb {
	case "ls":
		return ls(dir)
	case "get":
		v, err := get(dir, args[2])
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, v)
	case "set":
		key, value := args[2], args[3]
		if err := set(dir, key, value); err != nil {
			return err
		}
		fmt.Fprintf(stderr, "set %q\n", key)
	case "del":
		if err := del(dir, args[2]); err != nil {
			return err
		}
		fmt.Fprintf(stderr, "deleted %q\n", args[2])
	}
	return nil
}

func validate(args []string) (verb string, err error) {
	var errs []error
	if os.Getenv("SPKEZREPO") == "" {
		errs = append(errs, errors.New("SPKEZREPO not set"))
	}
	if os.Getenv("SPKEZPASS") == "" {
		errs = append(errs, errors.New("SPKEZPASS not set"))
	}
	if _, err := rnr.Get("git", "--version"); err != nil {
		errs = append(errs, errors.New("git not found"))
	}
	if len(args) < 2 {
		errs = append([]error{errUsage}, errs...)
	} else if verb = args[1]; !verbs[verb] {
		errs = append([]error{errUsage}, errs...)
	} else if (verb == "get" || verb == "del") && len(args) < 3 {
		errs = append([]error{errUsage}, errs...)
	} else if verb == "set" && len(args) < 4 {
		errs = append([]error{errUsage}, errs...)
	}
	return verb, errors.Join(errs...)
}

func repodir(url string) (string, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user cache directory: %w", err)
	}
	dir, err := repo.Clone(filepath.Join(cache, "spkez"), url, false)
	if err != nil {
		return "", fmt.Errorf("failed to fetch repository: %w", err)
	}
	if _, err := rnr.Get("git", "-C", dir, "pull", "--rebase"); err != nil {
		return "", fmt.Errorf("failed to fetch repository updates: %w", err)
	}
	return dir, nil
}

func ls(dir string) error {
	return filepath.WalkDir(
		dir,
		func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.Type().IsDir() && d.Name() == ".git" {
				return filepath.SkipDir
			}
			if d.Type().IsRegular() {
				if p, err := filepath.Rel(dir, path); err == nil {
					fmt.Fprintln(stdout, filepath.ToSlash(p))
				}
			}
			return nil
		},
	)
}

func get(dir, key string) (value string, err error) {
	path := filepath.Join(dir, filepath.FromSlash(key))
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file %q: %w", path, err)
	}
	defer f.Close()
	value, err = decrypt(f, os.Getenv("SPKEZPASS"))
	if err != nil {
		err = fmt.Errorf("failed to decrypt %q: %w", key, err)
	}
	return
}

func set(dir, key, value string) error {
	path := filepath.Join(dir, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create directory %q: %w",
			filepath.Dir(path), err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file %q: %w", path, err)
	}
	defer f.Close()
	if err = encrypt(f, value, os.Getenv("SPKEZPASS")); err != nil {
		return fmt.Errorf("failed to write %q: %w", key, err)
	}
	return pushfile(dir, key, fmt.Sprintf("set %q", key))
}

func del(dir, key string) error {
	path := filepath.Join(dir, filepath.FromSlash(key))
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("failed to delete %q: %w", key, err)
	}
	return pushfile(dir, key, fmt.Sprintf("del %q", key))
}

func pushfile(repo, path, msg string) error {
	_, err := rnr.Get("git", "-C", repo, "add", filepath.ToSlash(path))
	if err != nil {
		return fmt.Errorf("git add failed: %w", err)
	}
	r, err := rnr.Get("git", "-C", repo, "status", "-s")
	if err != nil {
		return fmt.Errorf("git status failed: %w", err)
	}
	if r.Out == "" {
		return nil // Nothing changed, nothing to commit.
	}
	_, err = rnr.Get("git", "-C", repo, "commit", "-am", msg)
	if err != nil {
		return fmt.Errorf("git commit failed: %w", err)
	}
	_, err = rnr.Get("git", "-C", repo, "push")
	if err != nil {
		return fmt.Errorf("git push failed: %w", err)
	}
	return nil
}

func key(pass string, salt []byte) []byte {
	return pbkdf2.Key([]byte(pass), salt, 4096, keysz, sha256.New)
}

func encrypt(w io.Writer, text, pass string) error {
	salt := make([]byte, saltsz)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return fmt.Errorf("failed to generate salt: %w", err)
	}

	block, err := aes.NewCipher(key(pass, salt))
	if err != nil {
		return fmt.Errorf("failed to create cipher block: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed to create gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("failed to generate nonce: %w", err)
	}
	var ctext []byte
	ctext = append(ctext, salt...)
	ctext = append(ctext, nonce...)
	ctext = append(ctext, gcm.Seal(nil, nonce, []byte(text), nil)...)

	encoder := base64.NewEncoder(base64.StdEncoding, w)
	defer encoder.Close()
	if _, err = encoder.Write(ctext); err != nil {
		return err
	}

	return nil
}

func decrypt(r io.Reader, pass string) (string, error) {
	decoder := base64.NewDecoder(base64.StdEncoding, r)
	ctext, err := io.ReadAll(decoder)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %w", err)
	}

	salt := ctext[:saltsz]
	ctext = ctext[saltsz:]

	block, err := aes.NewCipher(key(pass, salt))
	if err != nil {
		return "", fmt.Errorf("failed to create cipher block: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create gcm: %w", err)
	}

	nonce := ctext[:gcm.NonceSize()]
	ctext = ctext[gcm.NonceSize():]

	text, err := gcm.Open(nil, nonce, ctext, nil)
	if err != nil {
		return "", err
	}

	return string(text), nil
}

func boolmap[T comparable](v ...T) map[T]bool {
	m := make(map[T]bool)
	for _, e := range v {
		m[e] = true
	}
	return m
}
