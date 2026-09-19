package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	defaultTimeout = 5 * time.Second
	defaultRepo    = "Baaaki/PureSend"
)

// Endpoint URL can be overridden in tests.
var endpointURL = fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", defaultRepo)

// ReleaseInfo holds the details of a GitHub release.
type ReleaseInfo struct {
	TagName      string  `json:"tag_name"`
	Name         string  `json:"name"`
	HTMLURL      string  `json:"html_url"`
	Body         string  `json:"body"`
	Assets       []Asset `json:"assets"`
	HasUpdate    bool    `json:"-"`
	CleanVersion string  `json:"-"`
}

// Asset represents a downloadable binary or package in a GitHub release.
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// parseVersion extracts integer components from a semver string (e.g. "v0.3.1" -> [0, 3, 1]).
func parseVersion(v string) ([]int, bool) {
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	v = strings.TrimSpace(v)
	if idx := strings.IndexAny(v, "-+"); idx != -1 {
		v = v[:idx]
	}
	if v == "" {
		return nil, false
	}
	parts := strings.Split(v, ".")
	nums := make([]int, len(parts))
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil, false
		}
		nums[i] = n
	}
	return nums, true
}

// IsNewer reports whether the latest version is strictly newer than current.
func IsNewer(latest, current string) bool {
	latestNums, okLatest := parseVersion(latest)
	if !okLatest {
		return false
	}
	currentNums, okCurrent := parseVersion(current)
	if !okCurrent {
		// Non-semver current (e.g. "dev", "unknown") is considered older than a valid release.
		return true
	}
	maxLen := max(len(latestNums), len(currentNums))
	for i := 0; i < maxLen; i++ {
		l := 0
		if i < len(latestNums) {
			l = latestNums[i]
		}
		c := 0
		if i < len(currentNums) {
			c = currentNums[i]
		}
		if l > c {
			return true
		}
		if l < c {
			return false
		}
	}
	return false
}

// CheckLatest queries GitHub for the latest release and compares it with currentVersion.
func CheckLatest(ctx context.Context, currentVersion string) (*ReleaseInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpointURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	ua := "puresend"
	if currentVersion != "" {
		ua += "/" + currentVersion
	}
	req.Header.Set("User-Agent", ua)

	client := &http.Client{Timeout: defaultTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return &ReleaseInfo{HasUpdate: false}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api returned status %d", resp.StatusCode)
	}

	var info ReleaseInfo
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&info); err != nil {
		return nil, fmt.Errorf("failed to parse release: %w", err)
	}

	info.CleanVersion = strings.TrimPrefix(info.TagName, "v")
	info.HasUpdate = IsNewer(info.TagName, currentVersion)
	return &info, nil
}

// FindAsset finds the most appropriate binary asset for the given GOOS and GOARCH.
func FindAsset(assets []Asset, targetOS, targetArch string) *Asset {
	osLower := strings.ToLower(targetOS)
	archLower := strings.ToLower(targetArch)

	osAliases := []string{osLower}
	if osLower == "darwin" {
		osAliases = append(osAliases, "macos", "osx", "mac")
	} else if osLower == "windows" {
		osAliases = append(osAliases, "win")
	}

	archAliases := []string{archLower}
	if archLower == "amd64" {
		archAliases = append(archAliases, "x86_64", "x64", "64bit")
	} else if archLower == "arm64" {
		archAliases = append(archAliases, "aarch64")
	}

	matchOS := func(name string) bool {
		for _, alias := range osAliases {
			if strings.Contains(name, alias) {
				return true
			}
		}
		return false
	}

	matchArch := func(name string) bool {
		for _, alias := range archAliases {
			if strings.Contains(name, alias) {
				return true
			}
		}
		return false
	}

	for i := range assets {
		name := strings.ToLower(assets[i].Name)
		// Avoid source tarballs, checksum files, and deb packages
		if strings.HasSuffix(name, ".sha256") || strings.HasSuffix(name, ".md5") || strings.Contains(name, "source") || strings.HasSuffix(name, ".deb") {
			continue
		}
		if matchOS(name) && matchArch(name) {
			return &assets[i]
		}
	}
	return nil
}

// extractBinary writes the target binary to dst. If assetName is a tar.gz or zip, it extracts the binary entry.
func extractBinary(assetName string, r io.Reader, dst io.Writer) error {
	lower := strings.ToLower(assetName)
	if strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz") {
		gr, err := gzip.NewReader(r)
		if err != nil {
			return fmt.Errorf("gzip arşivi okunamadı: %w", err)
		}
		defer gr.Close()

		tr := tar.NewReader(gr)
		for {
			hdr, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return fmt.Errorf("tar arşivi okunamadı: %w", err)
			}
			if hdr.Typeflag == tar.TypeReg {
				base := filepath.Base(hdr.Name)
				if base == "puresend" || base == "puresend.exe" {
					_, err := io.Copy(dst, tr)
					return err
				}
			}
		}
		return errors.New("arşiv içinde puresend çalıştırılabilir dosyası bulunamadı")
	}

	if strings.HasSuffix(lower, ".zip") {
		buf, err := io.ReadAll(r)
		if err != nil {
			return fmt.Errorf("zip indirilemedi: %w", err)
		}
		zr, err := zip.NewReader(bytes.NewReader(buf), int64(len(buf)))
		if err != nil {
			return fmt.Errorf("zip arşivi okunamadı: %w", err)
		}
		for _, f := range zr.File {
			base := filepath.Base(f.Name)
			if base == "puresend" || base == "puresend.exe" {
				rc, err := f.Open()
				if err != nil {
					return err
				}
				defer rc.Close()
				_, err = io.Copy(dst, rc)
				return err
			}
		}
		return errors.New("zip arşivi içinde puresend çalıştırılabilir dosyası bulunamadı")
	}

	// Plain binary
	_, err := io.Copy(dst, r)
	return err
}

// Apply checks for updates and replaces the running executable if a newer release exists.
func Apply(currentVersion string, stdout io.Writer) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	fmt.Fprintln(stdout, "Güncellemeler kontrol ediliyor...")
	info, err := CheckLatest(ctx, currentVersion)
	if err != nil {
		return fmt.Errorf("sürüm kontrolü yapılamadı: %w", err)
	}

	if !info.HasUpdate {
		fmt.Fprintf(stdout, "PureSend zaten güncel (%s).\n", currentVersion)
		return nil
	}

	fmt.Fprintf(stdout, "Yeni bir sürüm mevcut: %s (mevcut sürüm: %s)\n", info.TagName, currentVersion)

	asset := FindAsset(info.Assets, runtime.GOOS, runtime.GOARCH)
	if asset == nil {
		fmt.Fprintf(stdout, "Sisteminiz için (%s/%s) hazır ikili dosya bulunamadı.\n", runtime.GOOS, runtime.GOARCH)
		fmt.Fprintf(stdout, "Yeni sürümü buradan indirebilirsiniz:\n  %s\n", info.HTMLURL)
		return nil
	}

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("çalışan dosya konumu belirlenemedi: %w", err)
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return fmt.Errorf("dosya yolu çözümlenemedi: %w", err)
	}

	dir := filepath.Dir(exePath)
	tmpFile, err := os.CreateTemp(dir, "ft-update-*")
	if err != nil {
		if errors.Is(err, os.ErrPermission) {
			return fmt.Errorf("yazma yetkisi yok (yönetici izinleriyle veya 'sudo' ile deneyin): %w", err)
		}
		return fmt.Errorf("geçici dosya oluşturulamadı: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	fmt.Fprintf(stdout, "%s indiriliyor...\n", asset.Name)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.BrowserDownloadURL, nil)
	if err != nil {
		tmpFile.Close()
		return err
	}
	req.Header.Set("User-Agent", "puresend/"+currentVersion)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		tmpFile.Close()
		return fmt.Errorf("indirme başarısız: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		tmpFile.Close()
		return fmt.Errorf("indirme sunucusu hata kodu döndürdü: %d", resp.StatusCode)
	}

	if err := extractBinary(asset.Name, resp.Body, tmpFile); err != nil {
		tmpFile.Close()
		return fmt.Errorf("dosya yazılamadı: %w", err)
	}
	if err := tmpFile.Chmod(0o755); err != nil {
		tmpFile.Close()
		return fmt.Errorf("çalıştırma izni verilemedi: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}

	if runtime.GOOS == "windows" {
		oldPath := exePath + ".old"
		_ = os.Remove(oldPath)
		if err := os.Rename(exePath, oldPath); err != nil {
			return fmt.Errorf("eski dosya yeniden adlandırılamadı: %w", err)
		}
	}

	if err := os.Rename(tmpPath, exePath); err != nil {
		return fmt.Errorf("yeni sürüm yüklenemedi: %w", err)
	}

	fmt.Fprintf(stdout, "PureSend başarıyla %s sürümüne güncellendi!\n", info.TagName)
	return nil
}
