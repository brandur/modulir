package mfile

import (
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/brandur/modulir"
	gocache "github.com/patrickmn/go-cache"
	"golang.org/x/xerrors"
)

//////////////////////////////////////////////////////////////////////////////
//
//
//
// Public
//
//
//
//////////////////////////////////////////////////////////////////////////////

// CopyFile is a shortcut for copy a file from a source path to a target path.
func CopyFile(c *modulir.Context, source, target string) error {
	in, err := os.Open(source)
	if err != nil {
		return xerrors.Errorf("error opening copy source: %w", err)
	}
	defer in.Close()

	out, err := os.Create(target)
	if err != nil {
		return xerrors.Errorf("error creating copy target: %w", err)
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return xerrors.Errorf("error copying data: %w", err)
	}

	c.Log.Debugf("mfile: Copied '%s' to '%s'", source, target)
	return nil
}

// CopyFileToDir is a shortcut for copy a file from a source path to a target
// directory.
func CopyFileToDir(c *modulir.Context, source, targetDir string) error {
	return CopyFile(c, source, path.Join(targetDir, filepath.Base(source)))
}

// EnsureDir ensures the existence of a target directory.
func EnsureDir(c *modulir.Context, target string) error {
	err := os.MkdirAll(target, 0755)
	if err != nil {
		return xerrors.Errorf("error creating directory: %w", err)
	}

	c.Log.Debugf("mfile: Ensured dir existence: %s", target)
	return nil
}

// EnsureSymlink ensures the existence of a symbolic link that maps a target
// path to a source path.
func EnsureSymlink(c *modulir.Context, source, target string) error {
	c.Log.Debugf("Checking symbolic link (%v): %v -> %v",
		path.Base(source), source, target)

	var actual string

	_, err := os.Stat(target)

	// Note that if a symlink file does exist, but points to a non-existent
	// location, we still get an "does not exist" error back, so we fall down
	// to the general create path so that the symlink file can be removed.
	//
	// The call to RemoveAll does not affect the other path of the symlink file
	// not being present because it doesn't care whether or not the file it's
	// trying remove is actually there.
	if os.IsNotExist(err) {
		c.Log.Debugf("Destination link does not exist. Creating.")
		goto create
	}
	if err != nil {
		return xerrors.Errorf("error checking symlink: %w", err)
	}

	actual, err = os.Readlink(target)
	if err != nil {
		return xerrors.Errorf("error reading symlink: %w", err)
	}

	if actual == source {
		c.Log.Debugf("Link exists.")
		return nil
	}

	c.Log.Debugf("Destination links to wrong source. Creating.")

create:
	err = os.RemoveAll(target)
	if err != nil {
		return xerrors.Errorf("error removing symlink: %w", err)
	}

	source, err = filepath.Abs(source)
	if err != nil {
		return err
	}

	target, err = filepath.Abs(target)
	if err != nil {
		return err
	}

	err = os.Symlink(source, target)
	if err != nil {
		return xerrors.Errorf("error creating symlink: %w", err)
	}

	return nil
}

// IsBackup indicates whether a given filename is a backup file (i.e. prefixed
// by `~`).
func IsBackup(base string) bool {
	return strings.HasSuffix(base, "~")
}

// IsHidden indicates whether a given filename is a hidden file (i.e. prefixed
// by `.`).
func IsHidden(base string) bool {
	return strings.HasPrefix(base, ".")
}

// IsMeta indicates whether a given filename is a "meta" file (i.e. prefixed by
// `_`).
func IsMeta(base string) bool {
	return strings.HasPrefix(base, "_")
}

// Exists is a shortcut to check if a file exists. It panics if encountering an
// unexpected error.
func Exists(file string) bool {
	_, err := os.Stat(file)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	panic(err)
}

// MustAbs is a shortcut variant of filepath.Abs which panics instead of
// returning an error.
func MustAbs(path string) string {
	absPath, err := filepath.Abs(path)
	if err != nil {
		panic(err)
	}
	return absPath
}

//
// ReadDir
//

// ReadDir reads files in a directory and returns a list of file paths.
//
// Unlike ioutil.ReadDir, this function skips hidden, "meta" (i.e. prefixed by
// an underscore), and Vim backup (i.e. suffixed with a tilde) files, and
// returns a list of full paths (easier to plumb into other functions), and
// sets up a watch on the listed source.
func ReadDir(c *modulir.Context, source string) ([]string, error) {
	return ReadDirWithOptions(c, source, nil)
}

// ReadDirOptions are options for ReadDirWithOptions.
type ReadDirOptions struct {
	// ShowBackup tells the function to not skip backup files like those
	// produced by Vim. These are suffixed with a tilde '~'.
	ShowBackup bool

	// ShowDirs tell the function not to skip directories.
	ShowDirs bool

	// ShowHidden tells the function to not skip hidden files (prefixed with a
	// dot '.').
	ShowHidden bool

	// ShowMeta tells the function to not skip so-called "meta" files
	// (prefixed with an underscore '_').
	ShowMeta bool
}

// ReadDirCached is the same as ReadDirWithOptions, but it caches results for
// some amount of time to make it faster. The downside of this of course is
// that we occasionally get a stale cache when a new file is added and don't
// see it.
func ReadDirCached(c *modulir.Context, source string,
	opts *ReadDirOptions) ([]string, error) {

	// Try to use a result from an expiring cache to speed up build loops that
	// run within close proximity of each other. Listing files is one of the
	// slower operations throughout the build loop, so this helps speed it up
	// quite a bit.
	//
	// Note that we only use the source as cache key even though technically
	// options could vary, which could potentially cause trouble. We know in
	// this project that ReadDir on particular directories always use the same
	// options, so we let that slide even if it's somewhat dangerous.
	if paths, ok := readDirCache.Get(source); ok {
		c.Log.Debugf("Using cached results of ReadDir: %s", source)
		return paths.([]string), nil
	}

	files, err := ReadDirWithOptions(c, source, opts)
	if err != nil {
		return nil, err
	}

	readDirCache.Set(source, files, gocache.DefaultExpiration)
	return files, nil
}

// ReadDirWithOptions reads files in a directory and returns a list of file
// paths.
//
// Unlike ReadDir, its behavior can be tweaked.
func ReadDirWithOptions(c *modulir.Context, source string,
	opts *ReadDirOptions) ([]string, error) {

	infos, err := ioutil.ReadDir(source)
	if err != nil {
		return nil, xerrors.Errorf("error reading directory: %w", err)
	}

	var files []string

	for _, info := range infos {
		base := filepath.Base(info.Name())

		if (opts == nil || !opts.ShowBackup) && IsBackup(base) {
			continue
		}

		if (opts == nil || !opts.ShowDirs) && info.IsDir() {
			continue
		}

		if (opts == nil || !opts.ShowHidden) && IsHidden(base) {
			continue
		}

		if (opts == nil || !opts.ShowMeta) && IsMeta(base) {
			continue
		}

		files = append(files, path.Join(source, info.Name()))
	}

	c.Log.Debugf("mfile: Read dir: %s", source)
	return files, nil
}

//////////////////////////////////////////////////////////////////////////////
//
//
//
// Private
//
//
//
//////////////////////////////////////////////////////////////////////////////

// An expiring cache that stores the results of a `mfile.ReadDir` (i.e. list
// directory) for some period of time. It turns out these calls are relatively
// slow and this helps speed up the build loop.
//
// The downside is that new files are not discovered right away, and often
// necessitate a server restart. A future improvement might be to have Modulir
// provide a simplified events channel that we can listen to in order to expire
// entries from the cache.
//
// Arguments are (defaultExpiration, cleanupInterval).
var readDirCache = gocache.New(5*time.Minute, 10*time.Minute)
