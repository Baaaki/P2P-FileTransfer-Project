// Package transfer implements the protocol that moves files directly
// between two peers. The rendezvous server plays no part in this stage.
//
// The flow: the receiver opens a stream to the sender. The sender first
// writes a manifest (file names, sizes and SHA-256 digests) and waits
// for the receiver to accept or decline. Only then does it stream the
// raw bytes of each file in order. The receiver verifies each digest
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
	"time"
)

// ProtocolID identifies the file transfer protocol on libp2p.
// 1.1.0 added the accept/decline step after the manifest.
const ProtocolID = "/puresend/transfer/1.1.0"

// maxManifestBytes caps the manifest line so a malicious sender cannot
// exhaust the receiver's memory before the manifest is even parsed.
const maxManifestBytes = 1 << 20

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

// ack is the receiver's answer to the manifest (accept/decline) and its
// confirmation at the end of the transfer.
type ack struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// ProgressFunc is called to report progress during the transfer.
type ProgressFunc func(fileName string, done, total int64)

// Send writes the given files to the stream. It runs inside the sending
// side's stream handler, which fires when the receiver opens the stream.
// The stream is any bidirectional byte pipe (a libp2p stream in practice,
// a net.Pipe in tests).
func Send(s io.ReadWriteCloser, paths []string, onProgress ProgressFunc) error {
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
	if err := w.Flush(); err != nil {
		return fmt.Errorf("could not send manifest: %w", err)
	}

	// The receiver inspects the manifest and explicitly accepts before
	// a single file byte is sent. Only acks flow in this direction, so
	// one decoder can safely be reused for both messages.
	dec := json.NewDecoder(s)
	var goAhead ack
	if err := dec.Decode(&goAhead); err != nil {
		return fmt.Errorf("no answer from receiver: %w", err)
	}
	if !goAhead.OK {
		return fmt.Errorf("receiver declined: %s", goAhead.Error)
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
	if err := dec.Decode(&a); err != nil {
		return fmt.Errorf("no acknowledgement from receiver: %w", err)
	}
	if !a.OK {
		return fmt.Errorf("error on the receiving side: %s", a.Error)
	}
	return nil
}

// Receive saves the files arriving on the stream into outDir and returns
// the paths of the saved files. Before anything is written, confirm is
// called with the manifest; returning false declines the transfer
// (a nil confirm accepts everything).
func Receive(s io.ReadWriteCloser, outDir string, confirm func(Manifest) bool, onProgress ProgressFunc) ([]string, error) {
	defer s.Close()

	// Read the manifest as a single line. Careful: json.Decoder cannot
	// be used here; it buffers extra data from the stream internally and
	// would swallow the beginning of the file bytes that follow.
	r := bufio.NewReader(s)
	line, err := readLimitedLine(r, maxManifestBytes)
	if err != nil {
		return nil, fmt.Errorf("could not read manifest: %w", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(line, &manifest); err != nil {
		return nil, fmt.Errorf("could not parse manifest: %w", err)
	}

	// Let the user inspect what is coming — name and size of every
	// file — and answer the sender before any disk space is used.
	if confirm != nil && !confirm(manifest) {
		json.NewEncoder(s).Encode(ack{OK: false, Error: "receiver declined the transfer"})
		return nil, fmt.Errorf("transfer declined")
	}
	if err := json.NewEncoder(s).Encode(ack{OK: true}); err != nil {
		return nil, fmt.Errorf("could not answer the sender: %w", err)
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, fmt.Errorf("could not create target directory: %w", err)
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

	// Linger until the sender closes the stream — it only does that
	// after reading our ack. Returning (and closing) right away lets
	// the process exit before the ack is actually transmitted, and the
	// sender would wait for it in vain.
	if d, ok := s.(interface{ SetReadDeadline(time.Time) error }); ok {
		d.SetReadDeadline(time.Now().Add(10 * time.Second))
	}
	io.Copy(io.Discard, io.LimitReader(r, 1))
	return saved, nil
}

// readLimitedLine reads one \n-terminated line of at most max bytes,
// unlike bufio.Reader.ReadBytes which buffers without any bound.
func readLimitedLine(r *bufio.Reader, max int) ([]byte, error) {
	var line []byte
	for {
		chunk, err := r.ReadSlice('\n')
		line = append(line, chunk...)
		if len(line) > max {
			return nil, fmt.Errorf("manifest exceeds %d bytes", max)
		}
		if err == nil {
			return line, nil
		}
		if err != bufio.ErrBufferFull {
			return nil, err
		}
	}
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

	// Write into a temporary .part file first; the final name only
	// appears after the digest has been verified, so an interrupted
	// transfer can never leave a corrupt file that looks complete.
	f, err := os.CreateTemp(outDir, filepath.Base(path)+".*.part")
	if err != nil {
		return "", fmt.Errorf("could not create file: %w", err)
	}
	tmp := f.Name()
	defer func() {
		f.Close()
		os.Remove(tmp) // no-op once the file has been renamed
	}()

	hasher := sha256.New()
	buf := make([]byte, 32*1024)
	var received int64
	for received < info.Size {
		chunk := min(int64(len(buf)), info.Size-received)
		// io.ReadFull tolerates a Read that returns the final bytes
		// together with io.EOF; a bare Read loop would misreport that
		// as a lost connection.
		n, err := io.ReadFull(r, buf[:chunk])
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				return "", fmt.Errorf("could not write to disk: %w", werr)
			}
			hasher.Write(buf[:n])
			received += int64(n)
			if onProgress != nil {
				onProgress(info.Name, received, info.Size)
			}
		}
		if err != nil {
			return "", fmt.Errorf("connection lost while receiving %s: %w", info.Name, err)
		}
	}

	if got := hex.EncodeToString(hasher.Sum(nil)); got != info.SHA256 {
		return "", fmt.Errorf("checksum mismatch for %s, the file may be corrupted", info.Name)
	}
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("could not finish writing %s: %w", info.Name, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return "", fmt.Errorf("could not finalize %s: %w", info.Name, err)
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
