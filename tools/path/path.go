package path

import (
	"os"
	"path/filepath"
	"strings"

	doublestar "github.com/bmatcuk/doublestar/v4"
)

type FilePath string

// New returns a file path. if you call some method on filePath object,
// will not change its original content
func New(path string) FilePath {
	return FilePath(path)
}

// String returns the path as a string.
func (f FilePath) String() string {
	return string(f)
}

// Abs returns an absolute representation of path.
func (f FilePath) Abs() (FilePath, error) {
	fi, err := filepath.Abs(string(f))
	return FilePath(fi), err
}

// Base returns the last element of path.
func (f FilePath) Base() string {
	return filepath.Base(string(f))
}

// Dir returns the directory part of path.
func (f FilePath) Dir() FilePath {
	return FilePath(filepath.Dir(string(f)))
}

// Ext returns the file name extension.
func (f FilePath) Ext() string {
	return filepath.Ext(string(f))
}

// Join joins any number of path elements into a single path, adding a separator
func (f FilePath) Join(path ...string) FilePath {
	pt := make([]string, len(path)+1)
	pt[0] = string(f)
	_ = copy(pt[1:], path)
	return FilePath(filepath.Join(pt...))
}

// Rel returns the f's relative path from base.
func (f FilePath) Rel(base string) (FilePath, error) {
	fi, err := filepath.Rel(base, string(f))
	return FilePath(fi), err
}

// IsAbs returns true if the path is absolute.
func (f FilePath) IsAbs() bool {
	return filepath.IsAbs(string(f))
}

// IsDir returns true if the path is a directory.
func (f FilePath) IsDir() bool {
	fi, err := os.Stat(string(f))
	return err == nil && fi.IsDir()
}

// IsFile returns true if the path is a file.
func (f FilePath) IsFile() bool {
	fi, err := os.Stat(string(f))
	return err == nil && !fi.IsDir()
}

// IsExist returns true if the path exists.
func (f FilePath) IsExist() bool {
	_, err := os.Stat(string(f))
	return err == nil
}

// IsGitRepo returns true if the path is a git repo.
func (f FilePath) IsGitRepo() bool {
	fi, err := os.Stat(f.Join(".git").String())
	return err == nil && fi.IsDir()
}

// HasExt returns true if the path has the given extension.
func (f FilePath) HasExt(ext string) bool {
	return strings.HasSuffix(string(f), ext)
}

type walkFunc func(FilePath) error

func (f FilePath) Walk(fn walkFunc) error {
	return filepath.Walk(string(f),
		func(path string, info os.FileInfo, err error) error {
			return fn(FilePath(path))
		})
}

// Match returns true if `f` matches the file name `pattern`
func (f FilePath) Match(pattern string) bool {
	isMatch, _ := doublestar.PathMatch(pattern, string(f))
	return isMatch
}
