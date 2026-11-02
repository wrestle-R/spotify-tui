// Package update checks the stable GitHub release and activates a verified pair
// of installed binaries. It never touches account or provider storage.
package update

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"debug/elf"
	"debug/macho"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const Repository = "wrestle-R/spotify-tui-rdp"

var stable = regexp.MustCompile(`^v?([0-9]+)\.([0-9]+)\.([0-9]+)$`)

type Release struct {
	Tag     string `json:"tag_name"`
	Draft   bool   `json:"draft"`
	Preview bool   `json:"prerelease"`
	Assets  []struct {
		Name string `json:"name"`
	} `json:"assets"`
}

func (r Release) Version() string { return strings.TrimPrefix(r.Tag, "v") }
func (r Release) Archive() string { return archiveName(r.Version(), runtime.GOOS, runtime.GOARCH) }

type Installation struct{ Version, Executable string }
type Client struct {
	Current, Executable string
	Run                 func(context.Context, string, ...string) ([]byte, error)
	Fetch               func(context.Context, string, io.Writer, int64) error
}

func New(version string) *Client {
	exe, _ := os.Executable()
	return &Client{Current: version, Executable: exe, Run: run, Fetch: fetch}
}

type cappedBuffer struct {
	data []byte
	max  int
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	if len(b.data)+len(p) > b.max {
		return 0, errors.New("command output exceeded limit")
	}
	b.data = append(b.data, p...)
	return len(p), nil
}
func run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	buf := &cappedBuffer{max: 1 << 20}
	cmd.Stdout = buf
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	return buf.data, nil
}
func versionParts(v string) ([3]int, error) {
	var p [3]int
	m := stable.FindStringSubmatch(v)
	if m == nil {
		return p, errors.New("expected a stable version")
	}
	for i := range p {
		n, e := strconv.Atoi(m[i+1])
		if e != nil {
			return p, e
		}
		p[i] = n
	}
	return p, nil
}
func Newer(a, b string) bool {
	x, e := versionParts(a)
	y, f := versionParts(b)
	if e != nil || f != nil {
		return false
	}
	for i := range x {
		if x[i] != y[i] {
			return x[i] > y[i]
		}
	}
	return false
}
func fetch(ctx context.Context, url string, out io.Writer, limit int64) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Spotify-Tui-updater")
	req.Header.Set("Accept", "application/vnd.github+json")
	response, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned HTTP %d", response.StatusCode)
	}
	n, err := io.Copy(out, io.LimitReader(response.Body, limit+1))
	if err != nil {
		return err
	}
	if n > limit {
		return errors.New("download exceeds size limit")
	}
	return nil
}
func (c *Client) Check(ctx context.Context) (*Release, error) {
	if !supportedTarget(runtime.GOOS, runtime.GOARCH) {
		return nil, errors.New("updates support Linux x86_64 and Apple Silicon macOS")
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	buf := &cappedBuffer{max: 1 << 20}
	var data []byte
	var err error
	if c.Fetch != nil {
		err = c.Fetch(ctx, "https://api.github.com/repos/"+Repository+"/releases/latest", buf, 1<<20)
		data = buf.data
	} else {
		err = errors.New("no HTTP client")
	}
	if err != nil {
		data, err = c.Run(ctx, "gh", "api", "--hostname", "github.com", "repos/"+Repository+"/releases/latest")
	}
	if err != nil {
		return nil, errors.New("update check unavailable; check your connection (private repositories also require gh auth login)")
	}

	var r Release
	if json.Unmarshal(data, &r) != nil || r.Draft || r.Preview || !stable.MatchString(r.Tag) {
		return nil, errors.New("GitHub did not return a stable release")
	}
	if !Newer(r.Tag, c.Current) {
		return nil, nil
	}
	required := map[string]bool{r.Archive(): false, "SHA256SUMS": false}
	for _, a := range r.Assets {
		if _, ok := required[a.Name]; ok {
			required[a.Name] = true
		}
	}
	for _, found := range required {
		if !found {
			return nil, errors.New("latest release is missing this platform archive or checksums")
		}
	}
	return &r, nil
}
func ManagedPrefix(executable string) (string, error) {
	exe, err := filepath.EvalSymlinks(executable)
	if err != nil {
		return "", err
	}
	app := filepath.Dir(filepath.Dir(exe))
	if filepath.Base(exe) != "spotify-tui" || filepath.Base(app) != "spotify-tui" || filepath.Base(filepath.Dir(app)) != "lib" {
		return "", errors.New("in-app updates need an installed release; run the downloaded install.sh first")
	}
	prefix := filepath.Dir(filepath.Dir(app))
	pairDir := ""
	for _, name := range []string{"spotify-tui", "spotify-tui-playback"} {
		link := filepath.Join(prefix, "bin", name)
		st, e := os.Lstat(link)
		if e != nil || st.Mode()&os.ModeSymlink == 0 {
			return "", errors.New("installed commands must be managed symlinks")
		}
		target, e := filepath.EvalSymlinks(link)
		if e != nil || !strings.HasPrefix(target, app+string(os.PathSeparator)) {
			return "", errors.New("refusing to replace an unrelated installed command")
		}
		if filepath.Base(target) != name {
			return "", errors.New("installed command target has the wrong name")
		}
		if pairDir != "" && pairDir != filepath.Dir(target) {
			return "", errors.New("installed binary pair is inconsistent")
		}
		pairDir = filepath.Dir(target)
	}
	return prefix, nil
}
func verifyChecksum(dir string, r Release) error {
	data, err := os.ReadFile(filepath.Join(dir, "SHA256SUMS"))
	if err != nil || len(data) > 1<<20 {
		return errors.New("missing or oversized release checksums")
	}
	expected := ""
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == r.Archive() {
			if expected != "" {
				return errors.New("duplicate archive checksum")
			}
			expected = fields[0]
		}
	}
	decoded, e := hex.DecodeString(expected)
	if e != nil || len(decoded) != 32 {
		return errors.New("invalid archive checksum")
	}
	file, err := os.Open(filepath.Join(dir, r.Archive()))
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha256.New()
	n, err := io.Copy(hash, io.LimitReader(file, 128<<20+1))
	if err != nil || n > 128<<20 {
		return errors.New("release archive exceeds size limit")
	}
	if !strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), expected) {
		return errors.New("release checksum mismatch; installed application was left intact")
	}
	return nil
}
func extract(archive, dir, root string) error {
	f, e := os.Open(archive)
	if e != nil {
		return e
	}
	defer f.Close()
	gz, e := gzip.NewReader(f)
	if e != nil {
		return e
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	var total int64
	for count := 0; ; count++ {
		h, e := tr.Next()
		if e == io.EOF {
			return nil
		}
		if e != nil {
			return e
		}
		if count > 2000 {
			return errors.New("too many archive entries")
		}
		name := strings.TrimSuffix(h.Name, "/")
		if name != path.Clean(name) || strings.HasPrefix(name, "/") || !(name == root || strings.HasPrefix(name, root+"/")) {
			return errors.New("unsafe archive path")
		}
		if h.Typeflag != tar.TypeReg && h.Typeflag != tar.TypeDir {
			return errors.New("links and special archive files are not allowed")
		}
		total += h.Size
		if h.Size < 0 || h.Size > 64<<20 || total > 200<<20 {
			return errors.New("unpacked release exceeds size limit")
		}
		target := filepath.Join(dir, filepath.FromSlash(name))
		if h.Typeflag == tar.TypeDir {
			if e = os.MkdirAll(target, 0700); e != nil {
				return e
			}
			continue
		}
		if e = os.MkdirAll(filepath.Dir(target), 0700); e != nil {
			return e
		}
		mode := os.FileMode(0600)
		if path.Base(name) == "spotify-tui" || path.Base(name) == "spotify-tui-playback" {
			mode = 0700
		}
		out, e := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
		if e != nil {
			return e
		}
		_, e = io.CopyN(out, tr, h.Size)
		closeErr := out.Close()
		if e != nil {
			return e
		}
		if closeErr != nil {
			return closeErr
		}
	}
}
func atomicLink(target, link string) error {
	f, e := os.CreateTemp(filepath.Dir(link), ".link-*")
	if e != nil {
		return e
	}
	tmp := f.Name()
	f.Close()
	os.Remove(tmp)
	defer os.Remove(tmp)
	if e = os.Symlink(target, tmp); e != nil {
		return e
	}
	return os.Rename(tmp, link)
}
func activate(prefix, stage string) error {
	app := filepath.Join(prefix, "lib", "spotify-tui")
	current := filepath.Join(app, "current")
	if st, e := os.Lstat(current); e == nil {
		if st.Mode()&os.ModeSymlink == 0 {
			return errors.New("current build pointer is not a symlink")
		}
		resolved, e := filepath.EvalSymlinks(current)
		if e != nil || !strings.HasPrefix(resolved, app+string(os.PathSeparator)) {
			return errors.New("invalid current build pointer")
		}
	} else if !os.IsNotExist(e) {
		return e
	} else {
		old, e := filepath.EvalSymlinks(filepath.Join(prefix, "bin", "spotify-tui"))
		if e != nil {
			return e
		}
		if e = atomicLink(filepath.Dir(old), current); e != nil {
			return e
		}
	}
	// Both command links use one pointer, so changing current activates the pair.
	for _, name := range []string{"spotify-tui", "spotify-tui-playback"} {
		if e := atomicLink(filepath.Join(current, name), filepath.Join(prefix, "bin", name)); e != nil {
			return e
		}
	}
	return atomicLink(stage, current)
}
func (c *Client) Install(ctx context.Context, r Release) (Installation, error) {
	var result Installation
	if !supportedTarget(runtime.GOOS, runtime.GOARCH) {
		return result, errors.New("updates support Linux x86_64 and Apple Silicon macOS")
	}
	if !stable.MatchString(r.Tag) || !Newer(r.Tag, c.Current) {
		return result, errors.New("only a newer stable release can be installed")
	}
	prefix, err := ManagedPrefix(c.Executable)
	if err != nil {
		return result, err
	}
	app := filepath.Join(prefix, "lib", "spotify-tui")
	lock, err := os.OpenFile(filepath.Join(app, ".update.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return result, err
	}
	defer lock.Close()
	if syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) != nil {
		return result, errors.New("another installation or update is already running")
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	temp, err := os.MkdirTemp(app, ".update-*")
	if err != nil {
		return result, err
	}
	defer os.RemoveAll(temp)
	downloaded := c.Fetch != nil
	if downloaded {
		for _, name := range []string{r.Archive(), "SHA256SUMS"} {
			f, e := os.OpenFile(filepath.Join(temp, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
			if e != nil {
				return result, e
			}
			limit := int64(128 << 20)
			if name == "SHA256SUMS" {
				limit = 1 << 20
			}
			e = c.Fetch(ctx, "https://github.com/"+Repository+"/releases/download/"+r.Tag+"/"+name, f, limit)
			closeErr := f.Close()
			if e != nil || closeErr != nil {
				downloaded = false
				break
			}
		}
	}
	if !downloaded {
		for _, name := range []string{r.Archive(), "SHA256SUMS"} {
			_ = os.Remove(filepath.Join(temp, name))
		}
		if _, err = c.Run(ctx, "gh", "release", "download", r.Tag, "--repo", Repository, "--pattern", r.Archive(), "--pattern", "SHA256SUMS", "--dir", temp); err != nil {
			return result, errors.New("release download failed; check connectivity and GitHub access")
		}
	}

	if err = verifyChecksum(temp, r); err != nil {
		return result, err
	}
	root := strings.TrimSuffix(r.Archive(), ".tar.gz")
	unpacked := filepath.Join(temp, "unpacked")
	if err = extract(filepath.Join(temp, r.Archive()), unpacked, root); err != nil {
		return result, err
	}
	source := filepath.Join(unpacked, root)
	for _, name := range []string{"spotify-tui", "spotify-tui-playback"} {
		binary := filepath.Join(source, name)
		if e := verifyExecutable(binary, runtime.GOOS, runtime.GOARCH); e != nil {
			return result, fmt.Errorf("%s: %w", name, e)
		}
		args := []string{"--version"}
		if name == "spotify-tui" {
			args = []string{"version"}
		}
		data, e := c.Run(ctx, binary, args...)
		if e != nil || strings.TrimSpace(string(data)) != name+" "+r.Version() {
			return result, errors.New("release binary versions do not match its tag")
		}
	}
	if err = ctx.Err(); err != nil {
		return result, err
	}
	stage, err := os.MkdirTemp(app, "build."+r.Version()+"-*")
	if err != nil {
		return result, err
	}
	keep := false
	defer func() {
		if !keep {
			os.RemoveAll(stage)
		}
	}()
	if err = os.Remove(stage); err != nil {
		return result, err
	}
	if err = os.Rename(source, stage); err != nil {
		return result, err
	}
	if err = activate(prefix, stage); err != nil {
		return result, err
	}
	keep = true
	return Installation{Version: r.Version(), Executable: filepath.Join(prefix, "bin", "spotify-tui")}, nil
}

func supportedTarget(goos, arch string) bool {
	return goos == "linux" && arch == "amd64" || goos == "darwin" && arch == "arm64"
}
func archiveName(version, goos, arch string) string {
	return "spotify-tui_" + version + "_" + goos + "_" + arch + ".tar.gz"
}
func verifyExecutable(path, goos, arch string) error {
	if !supportedTarget(goos, arch) {
		return errors.New("unsupported executable platform")
	}
	valid := false
	if goos == "darwin" {
		f, err := macho.Open(path)
		if err != nil {
			return errors.New("invalid macOS Mach-O executable")
		}
		defer f.Close()
		valid = f.Cpu == macho.CpuArm64 && f.Type == macho.TypeExec
	} else {
		f, err := elf.Open(path)
		if err != nil {
			return errors.New("invalid Linux ELF executable")
		}
		defer f.Close()
		valid = f.Machine == elf.EM_X86_64 && f.Class == elf.ELFCLASS64 && (f.Type == elf.ET_EXEC || f.Type == elf.ET_DYN)
	}
	if !valid {
		return errors.New("release executable architecture mismatch")
	}
	return nil
}
