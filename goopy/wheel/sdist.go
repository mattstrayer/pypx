package wheel

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"path"
	"slices"
	"sort"
	"strings"
	"unicode"
)

// maxSdistInflated caps the whole inflated tar stream, including files that are
// read past and dropped. It bounds the CPU a gzip bomb can use. Only sources stay
// in memory, and MaxDecompressedTotal caps those.
const maxSdistInflated = 1024 * 1024 * 1024 // 1 GiB

// sdistSkippedDirs are top-level source tree directories that a wheel build does
// not install as API.
var sdistSkippedDirs = map[string]bool{
	"tests": true, "test": true, "docs": true, "doc": true, "examples": true, "benchmarks": true,
}

// sdistScripts are top-level files that run a build or a tool. They are not modules
// of the distribution.
var sdistScripts = map[string]bool{
	"setup": true, "conftest": true, "ez_setup": true, "distribute_setup": true,
}

// selectSdist picks one sdist deterministically: .tar.gz, then .zip, then any other
// format, each by lexicographically smallest filename.
func selectSdist(sdists []WheelFile) WheelFile {
	rank := func(f WheelFile) int {
		switch {
		case isTarGz(f.Filename):
			return 0
		case strings.HasSuffix(f.Filename, ".zip"):
			return 1
		}
		return 2
	}
	return slices.MinFunc(sdists, func(a, b WheelFile) int {
		if c := rank(a) - rank(b); c != 0 {
			return c
		}
		return strings.Compare(a.Filename, b.Filename)
	})
}

// isReadableSdist reports whether fetchSdist reads this format.
func isReadableSdist(filename string) bool {
	return isTarGz(filename) || strings.HasSuffix(filename, ".zip")
}

func isTarGz(filename string) bool {
	return strings.HasSuffix(filename, ".tar.gz") || strings.HasSuffix(filename, ".tgz")
}

// fetchSdist reads the Python sources of an sdist into memory. Nothing in the
// archive runs, and nothing reaches the disk. MaxSize applies to compressed bytes,
// because neither format is read by range here.
func (s *Source) fetchSdist(ctx context.Context, f WheelFile, name string) (*WheelContents, error) {
	if f.Size > s.MaxSize {
		return nil, fmt.Errorf("%d bytes (max %d): %w", f.Size, s.MaxSize, ErrTooLarge)
	}
	c := &sdistCollector{files: map[string][]byte{}, topLevel: map[string][]byte{}}
	var err error
	switch {
	case isTarGz(f.Filename):
		err = s.readSdistTar(ctx, f.URL, c)
	case strings.HasSuffix(f.Filename, ".zip"):
		err = s.readSdistZip(ctx, f.URL, c)
	default:
		err = fmt.Errorf("%s: unsupported sdist format: %w", f.Filename, ErrNoArtifact)
	}
	if err != nil {
		return nil, fmt.Errorf("reading sdist: %w", err)
	}
	return c.contents(name)
}

// readSdistTar streams a .tar.gz. A .tar.gz has no index, so bad data ends the read
// and the fetch fails. A truncated body fails the same way, so a partial result is
// never returned.
func (s *Source) readSdistTar(ctx context.Context, url string, c *sdistCollector) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned %d", resp.StatusCode)
	}

	gz, err := gzip.NewReader(&capReader{
		r: resp.Body, left: s.MaxSize,
		err: fmt.Errorf("download exceeds %d bytes: %w", s.MaxSize, ErrTooLarge),
	})
	if err != nil {
		return err
	}
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(&capReader{
		r: gz, left: maxSdistInflated,
		err: fmt.Errorf("sdist inflates past %d bytes: %w", int64(maxSdistInflated), ErrTooLarge),
	})
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if hdr.Typeflag == tar.TypeXGlobalHeader {
			continue // Metadata, and git archive names it pax_global_header.
		}
		kind := sdistOther
		switch hdr.Typeflag {
		case tar.TypeReg:
			kind = sdistRegular
		case tar.TypeDir:
			kind = sdistDir
		}
		if err := c.add(hdr.Name, kind, func() ([]byte, error) { return readCapped(tr) }); err != nil {
			return err
		}
	}
}

// readSdistZip downloads a whole .zip. A corrupt entry is skipped, as in a wheel.
func (s *Source) readSdistZip(ctx context.Context, url string, c *sdistCollector) error {
	data, err := s.download(ctx, url)
	if err != nil {
		return err
	}
	if int64(len(data)) > s.MaxSize {
		return fmt.Errorf("download exceeds %d bytes: %w", s.MaxSize, ErrTooLarge)
	}
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("opening zip: %w", err)
	}
	for _, f := range r.File {
		kind := sdistOther
		switch {
		case f.FileInfo().IsDir():
			kind = sdistDir
		case f.Mode().IsRegular():
			kind = sdistRegular
		}
		err := c.add(f.Name, kind, func() ([]byte, error) {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer func() { _ = rc.Close() }()
			return readCapped(rc)
		})
		if err != nil && !isEntryCorrupt(err) {
			return err
		}
	}
	return nil
}

// readCapped reads one entry up to the per-file cap.
func readCapped(r io.Reader) ([]byte, error) {
	src, err := io.ReadAll(io.LimitReader(r, MaxDecompressedFile+1))
	if err != nil {
		return nil, err
	}
	if len(src) > MaxDecompressedFile {
		return nil, fmt.Errorf("sdist file exceeds per-file decompression cap (%d bytes): %w", MaxDecompressedFile, ErrTooLarge)
	}
	return src, nil
}

// capReader fails with err once more than left bytes have passed through it.
type capReader struct {
	r    io.Reader
	left int64
	err  error
}

func (c *capReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.left -= int64(n)
	if c.left < 0 {
		return 0, c.err
	}
	return n, err
}

type sdistEntryKind int

const (
	sdistRegular sdistEntryKind = iota
	sdistDir
	// sdistOther is a symlink, hardlink, device or anything else that is not a
	// regular file. Its data is never read and its target is never followed.
	sdistOther
)

// sdistCollector applies the path rules and caps to each archive entry.
type sdistCollector struct {
	entries int
	total   int64
	files   map[string][]byte
	// topLevel holds the top_level.txt files of .egg-info directories, by path.
	topLevel map[string][]byte
	// wrapper is the first path segment of every entry so far. noWrapper is set
	// once an entry shows the archive has no single top directory.
	wrapper   string
	noWrapper bool
}

// add takes one entry. read returns its inflated bytes.
func (c *sdistCollector) add(name string, kind sdistEntryKind, read func() ([]byte, error)) error {
	c.entries++
	if c.entries > maxFileCount {
		return fmt.Errorf("sdist has too many entries (max %d): %w", maxFileCount, ErrTooLarge)
	}
	segs, ok := sdistPathSegments(name)
	if !ok {
		return fmt.Errorf("unsafe path %q: %w", name, ErrNoArtifact)
	}
	if len(segs) == 0 {
		return nil // The archive root itself, as "./".
	}
	if c.wrapper == "" {
		c.wrapper = segs[0]
	}
	if segs[0] != c.wrapper || (len(segs) == 1 && kind != sdistDir) {
		c.noWrapper = true
	}
	if kind != sdistRegular {
		return nil
	}

	// The read skips test and doc trees, so they cannot use up the caps.
	if len(segs) > 2 && sdistSkippedDirs[segs[1]] {
		return nil
	}
	if slices.Contains(segs, "__pycache__") {
		return nil
	}
	base := segs[len(segs)-1]
	isTopLevel := base == "top_level.txt" && len(segs) >= 2 && strings.HasSuffix(segs[len(segs)-2], ".egg-info")
	if !isTopLevel && !strings.HasSuffix(base, ".py") && !strings.HasSuffix(base, ".pyi") {
		return nil
	}

	src, err := read()
	if err != nil {
		return err
	}
	c.total += int64(len(src))
	if c.total > MaxDecompressedTotal {
		return fmt.Errorf("sdist exceeds total decompression budget (%d bytes): %w", MaxDecompressedTotal, ErrTooLarge)
	}
	key := strings.Join(segs, "/")
	if isTopLevel {
		c.topLevel[key] = src
	} else {
		c.files[key] = src
	}
	return nil
}

// sdistPathSegments splits an archive path. ok is false for an absolute path, a
// Windows path, or any path with a ".." segment. Empty and "." segments drop out,
// so "./pkg-1.0/a.py" and "pkg-1.0/a.py" are the same path.
func sdistPathSegments(name string) (segs []string, ok bool) {
	if strings.HasPrefix(name, "/") || strings.ContainsAny(name, "\\\x00") {
		return nil, false
	}
	for seg := range strings.SplitSeq(name, "/") {
		switch {
		case seg == "..", strings.HasSuffix(seg, ":"):
			return nil, false
		case seg != "" && seg != ".":
			segs = append(segs, seg)
		}
	}
	return segs, true
}

// contents turns the collected files into import-tree paths.
func (c *sdistCollector) contents(pkgName string) (*WheelContents, error) {
	strip := func(p string) string {
		if c.noWrapper {
			return p
		}
		_, rest, _ := strings.Cut(p, "/")
		return rest
	}

	tree := make(map[string][]byte, len(c.files))
	for p, src := range c.files {
		tree[strip(p)] = src
	}
	root := sdistSourceRoot(tree)

	files := map[string][]byte{}
	for p, src := range tree {
		rel, ok := strings.CutPrefix(p, root)
		if ok && sdistImportable(rel) {
			files[rel] = src
		}
	}

	declared := declaredTopLevel(c.topLevel, strip, root, files)
	if len(declared) == 0 {
		dropNonPackages(files)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("sdist holds no Python sources: %w", ErrNoArtifact)
	}

	contents := &WheelContents{Files: files, TopLevelPkgs: declared}
	if len(declared) == 0 {
		contents.TopLevelPkgs = inferTopLevel(files, pkgName)
	}
	return contents, nil
}

// sdistAltRoots are directories that some projects use in place of src, as with
// package_dir={"": "python3"}. They count only if the tree root holds no package.
var sdistAltRoots = []string{"lib", "python3", "python"}

// sdistSourceRoot returns the directory that holds the import tree, with a trailing
// slash: src if it holds a package, or a module while the tree root holds no
// package; then an alternative root; otherwise "". A directory that is itself a
// package, as with src/__init__.py, stays under the tree root.
func sdistSourceRoot(tree map[string][]byte) string {
	packages := map[string]bool{}
	var srcModule bool
	for p := range tree {
		first, rest, nested := strings.Cut(p, "/")
		if !nested {
			continue
		}
		if isInitFile(rest) {
			sub, _, inDir := strings.Cut(rest, "/")
			if !inDir {
				packages[first+"/"] = true // first/__init__.py
			} else if !sdistSkippedDirs[sub] {
				packages[first] = true // A package somewhere below first.
			}
		} else if sub, _, inDir := strings.Cut(rest, "/"); inDir && strings.HasSuffix(sub, stubDirSuffix) {
			packages[first] = true
		} else if !inDir && first == "src" && sdistImportable(rest) {
			srcModule = true
		}
	}
	isPackage := func(dir string) bool { return packages[dir+"/"] }
	rootPackage := false
	for dir := range packages {
		if name, ok := strings.CutSuffix(dir, "/"); ok && !sdistSkippedDirs[name] {
			rootPackage = true
		}
	}
	if !isPackage("src") && (packages["src"] || (srcModule && !rootPackage)) {
		return "src/"
	}
	if !rootPackage {
		for _, dir := range sdistAltRoots {
			if packages[dir] {
				return dir + "/"
			}
		}
	}
	return ""
}

func isInitFile(p string) bool {
	base := path.Base(p)
	return base == "__init__.py" || base == "__init__.pyi"
}

// sdistImportable reports whether a path under the source root can be a module of
// the distribution: its segments are Python identifiers, and it is outside the
// test and doc trees. A stub directory may carry the -stubs suffix.
func sdistImportable(rel string) bool {
	segs := strings.Split(rel, "/")
	for i, seg := range segs {
		if i == len(segs)-1 {
			seg = strings.TrimSuffix(strings.TrimSuffix(seg, ".pyi"), ".py")
		}
		if i == 0 && len(segs) > 1 {
			seg = strings.TrimSuffix(seg, stubDirSuffix)
		}
		if !isPythonIdentifier(seg) {
			return false
		}
	}
	if len(segs) > 1 {
		return !sdistSkippedDirs[segs[0]]
	}
	stem := strings.TrimSuffix(strings.TrimSuffix(segs[0], ".pyi"), ".py")
	return !sdistScripts[stem] && !isTestFileStem(stem)
}

func isTestFileStem(stem string) bool {
	return stem == "test" || stem == "tests" || strings.HasPrefix(stem, "test_") || strings.HasSuffix(stem, "_test")
}

func isPythonIdentifier(s string) bool {
	for i, r := range s {
		if r != '_' && !unicode.IsLetter(r) && (i == 0 || !unicode.IsDigit(r)) {
			return false
		}
	}
	return s != ""
}

// declaredTopLevel reads the top_level.txt of an .egg-info directory at the source
// root or the tree root, the same metadata a wheel carries in .dist-info. It keeps
// the names that match a kept file. A vendored .egg-info deeper down does not count.
func declaredTopLevel(topLevel map[string][]byte, strip func(string) string, root string, files map[string][]byte) []string {
	var names []string
	for p, data := range topLevel {
		if rel := strings.TrimPrefix(strip(p), root); strings.Count(rel, "/") != 1 {
			continue
		}
		for _, name := range parseTopLevelTxt(string(data)) {
			name = strings.TrimSuffix(name, stubDirSuffix)
			if hasTopLevelFiles(files, name) {
				names = append(names, name)
			}
		}
	}
	sort.Strings(names)
	return slices.Compact(names)
}

// dropNonPackages applies the layout rules when no metadata names the packages. If
// any top-level directory is a package, only such directories stay: loose top-level
// files are scripts and other directories are tools. If not, directories that hold a
// package further down stay, as namespace packages do. Otherwise any top-level
// modules stay, and directories go. A tree with neither is left as is, since it can
// only be implicit namespace packages.
func dropNonPackages(files map[string][]byte) {
	regular, namespace := map[string]bool{}, map[string]bool{}
	hasModule := false
	for p := range files {
		first, rest, nested := strings.Cut(p, "/")
		switch {
		case !nested:
			hasModule = true
		case strings.HasSuffix(first, stubDirSuffix), isInitFile(rest) && !strings.Contains(rest, "/"):
			regular[first] = true
		case isInitFile(rest):
			namespace[first] = true
		}
	}
	packages := regular
	if len(packages) == 0 {
		packages = namespace
	}
	for p := range files {
		first, _, nested := strings.Cut(p, "/")
		switch {
		case len(packages) > 0 && !packages[first], len(packages) == 0 && hasModule && nested:
			delete(files, p)
		}
	}
}
