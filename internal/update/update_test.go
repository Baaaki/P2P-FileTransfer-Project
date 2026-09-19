package update

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsNewer(t *testing.T) {
	tests := []struct {
		latest  string
		current string
		want    bool
	}{
		{"0.3.0", "0.2.0", true},
		{"0.2.1", "0.2.0", true},
		{"1.0.0", "0.9.9", true},
		{"v0.3.0", "v0.2.0", true},
		{"v0.3.0-rc1", "v0.2.0", true},
		{"0.2.0", "0.2.0", false},
		{"v0.2.0", "0.2.0", false},
		{"0.1.9", "0.2.0", false},
		{"v0.1.0", "v0.2.0", false},
		{"0.2.0", "dev", true},
		{"v1.0.0", "unknown", true},
		{"v1.0.0", "", true},
		{"invalid", "0.2.0", false},
	}

	for _, tt := range tests {
		got := IsNewer(tt.latest, tt.current)
		if got != tt.want {
			t.Errorf("IsNewer(%q, %q) = %v, want %v", tt.latest, tt.current, got, tt.want)
		}
	}
}

func TestFindAsset(t *testing.T) {
	assets := []Asset{
		{Name: "filetransferilla-checksums.txt.sha256"},
		{Name: "filetransferilla-linux-amd64", BrowserDownloadURL: "https://example.com/linux-amd64"},
		{Name: "filetransferilla-windows-amd64.exe", BrowserDownloadURL: "https://example.com/win-amd64"},
		{Name: "filetransferilla-darwin-arm64", BrowserDownloadURL: "https://example.com/mac-arm64"},
	}

	foundLinux := FindAsset(assets, "linux", "amd64")
	if foundLinux == nil || foundLinux.BrowserDownloadURL != "https://example.com/linux-amd64" {
		t.Errorf("FindAsset(linux, amd64) failed: %v", foundLinux)
	}

	foundMac := FindAsset(assets, "darwin", "arm64")
	if foundMac == nil || foundMac.BrowserDownloadURL != "https://example.com/mac-arm64" {
		t.Errorf("FindAsset(darwin, arm64) failed: %v", foundMac)
	}

	foundWin := FindAsset(assets, "windows", "amd64")
	if foundWin == nil || foundWin.BrowserDownloadURL != "https://example.com/win-amd64" {
		t.Errorf("FindAsset(windows, amd64) failed: %v", foundWin)
	}

	notFound := FindAsset(assets, "freebsd", "riscv64")
	if notFound != nil {
		t.Errorf("FindAsset(freebsd, riscv64) should be nil, got: %v", notFound)
	}
}

func TestCheckLatest(t *testing.T) {
	mockRelease := ReleaseInfo{
		TagName: "v0.3.0",
		Name:    "Version 0.3.0",
		HTMLURL: "https://github.com/Baaaki/P2P-FileTransfer-Project/releases/tag/v0.3.0",
		Assets: []Asset{
			{Name: "filetransferilla-linux-amd64", BrowserDownloadURL: "https://example.com/bin"},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockRelease)
	}))
	defer srv.Close()

	// Override endpoint for testing
	origEndpoint := endpointURL
	endpointURL = srv.URL
	defer func() { endpointURL = origEndpoint }()

	// Test with older version
	info, err := CheckLatest(context.Background(), "v0.2.0")
	if err != nil {
		t.Fatalf("CheckLatest: %v", err)
	}
	if !info.HasUpdate {
		t.Errorf("Expected HasUpdate=true for v0.2.0")
	}
	if info.TagName != "v0.3.0" {
		t.Errorf("Expected TagName=v0.3.0, got: %s", info.TagName)
	}

	// Test with current version
	info2, err := CheckLatest(context.Background(), "v0.3.0")
	if err != nil {
		t.Fatalf("CheckLatest: %v", err)
	}
	if info2.HasUpdate {
		t.Errorf("Expected HasUpdate=false for v0.3.0")
	}
}

func TestCheckLatest404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	origEndpoint := endpointURL
	endpointURL = srv.URL
	defer func() { endpointURL = origEndpoint }()

	info, err := CheckLatest(context.Background(), "dev")
	if err != nil {
		t.Fatalf("CheckLatest 404 should not error: %v", err)
	}
	if info.HasUpdate {
		t.Errorf("Expected HasUpdate=false on 404")
	}
}

func TestApplyUpToDate(t *testing.T) {
	mockRelease := ReleaseInfo{
		TagName: "v0.2.0",
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mockRelease)
	}))
	defer srv.Close()

	origEndpoint := endpointURL
	endpointURL = srv.URL
	defer func() { endpointURL = origEndpoint }()

	var buf bytes.Buffer
	err := Apply("v0.2.0", &buf)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("zaten güncel")) {
		t.Errorf("Expected 'zaten güncel' message, got: %s", buf.String())
	}
}
