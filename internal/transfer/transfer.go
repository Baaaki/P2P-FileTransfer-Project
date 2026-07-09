// Package transfer implements the protocol that moves files directly
// between two peers. The rendezvous server plays no part in this stage.
//
// The flow: the receiver opens a stream to the sender. The sender first
// writes a manifest (file names, sizes and SHA-256 digests), then streams
// the raw bytes of each file in order. The receiver verifies each digest
// while writing to disk and sends a final acknowledgement.
package transfer

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/libp2p/go-libp2p/core/network"
)

// ProtocolID identifies the file transfer protocol on libp2p.
const ProtocolID = "/puresend/transfer/1.0.0"

// FileInfo describes a single file in the manifest.
type FileInfo struct {
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

// Manifest lists the files about to be sent.
type Manifest struct {
	Files []FileInfo `json:"files"`
}

// ack is the receiver's confirmation sent at the end of the transfer.
type ack struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// ProgressFunc is called to report progress during the transfer.
type ProgressFunc func(fileName string, done, total int64)

// Send writes the given files to the stream. It runs inside the sending
// side's stream handler, which fires when the receiver opens the stream.
func Send(s network.Stream, paths []string, onProgress ProgressFunc) error {
	defer s.Close()

	// Build the manifest first: size and SHA-256 digest of every file.
	manifest := Manifest{}
	for _, p := range paths {
		info, err := fileInfo(p)
		if err != nil {
			return err
		}
		manifest.Files = append(manifest.Files, info)
	}

	w := bufio.NewWriter(s)
	if err := json.NewEncoder(w).Encode(manifest); err != nil {
		return fmt.Errorf("could not send manifest: %w", err)
	}

	// Send the raw bytes of each file, in manifest order.
	for i, p := range paths {
		if err := sendFile(w, p, manifest.Files[i], onProgress); err != nil {
			return err
		}
	}
	if err := w.Flush(); err != nil {
		return fmt.Errorf("could not finish sending: %w", err)
	}

	// Wait for the receiver's acknowledgement so we don't declare
	// success before the files are safely on disk over there.
	var a ack
	if err := json.NewDecoder(s).Decode(&a); err != nil {
		return fmt.Errorf("no acknowledgement from receiver: %w", err)
	}
	if !a.OK {
		return fmt.Errorf("error on the receiving side: %s", a.Error)
	}
	return nil
}

// Receive saves the files arriving on the stream into outDir and returns
// the paths of the saved files.
func Receive(s network.Stream, outDir string, onProgress ProgressFunc) ([]string, error) {
	defer s.Close()

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, fmt.Errorf("could not create target directory: %w", err)
	}

	// Read the manifest as a single line. Careful: json.Decoder cannot
	// be used here; it buffers extra data from the stream internally and
	// would swallow the beginning of the file bytes that follow.
	r := bufio.NewReader(s)
	line, err := r.ReadBytes('\n')
	if err != nil {
		return nil, fmt.Errorf("could not read manifest: %w", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(line, &manifest); err != nil {
		return nil, fmt.Errorf("could not parse manifest: %w", err)
	}

	var saved []string
	for _, f := range manifest.Files {
		path, err := receiveFile(r, outDir, f, onProgress)
		if err != nil {
			// Let the sender know, then propagate the error.
			json.NewEncoder(s).Encode(ack{OK: false, Error: err.Error()})
			return saved, err
		}
		saved = append(saved, path)
	}

	if err := json.NewEncoder(s).Encode(ack{OK: true}); err != nil {
		return saved, fmt.Errorf("could not send acknowledgement: %w", err)
	}
	return saved, nil
}

// fileInfo computes a file's size and SHA-256 digest.
func fileInfo(path string) (FileInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return FileInfo{}, fmt.Errorf("could not open file: %w", err)
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return FileInfo{}, err
	}
	if stat.IsDir() {
		return FileInfo{}, fmt.Errorf("%s is a directory, expected a file", path)
	}

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return FileInfo{}, fmt.Errorf("could not compute digest: %w", err)
	}

	return FileInfo{
		Name:   filepath.Base(path),
		Size:   stat.Size(),
		SHA256: hex.EncodeToString(h.Sum(nil)),
	}, nil
}

// sendFile writes a single file's bytes in chunks, reporting progress
// after each chunk.
func sendFile(w io.Writer, path string, info FileInfo, onProgress ProgressFunc) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("could not open file: %w", err)
	}
	defer f.Close()

	buf := make([]byte, 32*1024)
	var sent int64
	for sent < info.Size {
		n, err := f.Read(buf)
		if n > 0 {
			if _, werr := w.Write(buf[:n]); werr != nil {
				return fmt.Errorf("connection lost while sending %s: %w", info.Name, werr)
			}
			sent += int64(n)
			if onProgress != nil {
				onProgress(info.Name, sent, info.Size)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("could not read %s: %w", info.Name, err)
		}
	}
	if sent != info.Size {
		return fmt.Errorf("size of %s changed while sending (expected %d, read %d)", info.Name, info.Size, sent)
	}
	return nil
}

// receiveFile writes a single file to disk and verifies its SHA-256 digest.
func receiveFile(r io.Reader, outDir string, info FileInfo, onProgress ProgressFunc) (string, error) {
	// filepath.Base prevents path traversal: even if the other side
	// sends a name like "../../.bashrc", it cannot escape outDir.
	path := availablePath(outDir, filepath.Base(info.Name))

	f, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("could not create file: %w", err)
	}
	defer f.Close()

	hasher := sha256.New()
	buf := make([]byte, 32*1024)
	var received int64
	for received < info.Size {
		chunk := int64(len(buf))
		if remaining := info.Size - received; remaining < chunk {
			chunk = remaining
		}
		n, err := r.Read(buf[:chunk])
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				return path, fmt.Errorf("could not write to disk: %w", werr)
			}
			hasher.Write(buf[:n])
			received += int64(n)
			if onProgress != nil {
				onProgress(info.Name, received, info.Size)
			}
		}
		if err != nil {
			return path, fmt.Errorf("connection lost while receiving %s: %w", info.Name, err)
		}
	}

	if got := hex.EncodeToString(hasher.Sum(nil)); got != info.SHA256 {
		return path, fmt.Errorf("checksum mismatch for %s, the file may be corrupted", info.Name)
	}
	return path, nil
}

// availablePath picks a free name like "photo (1).jpg" if a file with the
// same name already exists; we never overwrite existing files.
func availablePath(dir, name string) string {
	path := filepath.Join(dir, name)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path
	}
	ext := filepath.Ext(name)
	base := name[:len(name)-len(ext)]
	for i := 1; ; i++ {
		path = filepath.Join(dir, fmt.Sprintf("%s (%d)%s", base, i, ext))
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return path
		}
	}
}
