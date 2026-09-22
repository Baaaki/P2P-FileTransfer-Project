package integration

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// The client is compiled once and shared by every test in this file;
// building it per test would dominate the run time.
var (
	clientOnce sync.Once
	clientPath string
	clientErr  error
)

// TestHeadlessSendAndReceive builds the real client binary and drives two
// copies of it against a real server — no pseudo-terminal, no reading room
// codes off a rendered screen.
//
// This is the test the report asked for: before headless mode, the only
// way to exercise the shipped program end to end was to type into a pty
// and scrape ANSI output, which is brittle enough that it was done by hand
// instead of in CI. Now the whole path — flags, server address, room code,
// digest check, exit status — is one ordinary test.
func TestHeadlessSendAndReceive(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a binary; skipped under -short")
	}

	bin := buildClient(t)
	_, _, serverAddr := newWSServer(t)

	srcDir := t.TempDir()
	want := map[string][]byte{
		"belge.txt": []byte("merhaba dünya"),
		"veri.bin":  bytes.Repeat([]byte("puresend"), 20_000),
	}
	var paths []string
	for name, data := range want {
		p := filepath.Join(srcDir, name)
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, p)
	}

	// The sender prints its room code on stdout, alone on a line, and then
	// waits. That contract is what makes the mode scriptable.
	sendCmd := clientCmd(bin, "-server", serverAddr, "-send", strings.Join(paths, ","))
	stdout, err := sendCmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var sendLog bytes.Buffer
	sendCmd.Stderr = &sendLog
	if err := sendCmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sendCmd.Process.Kill() })

	room := readLine(t, stdout, 60*time.Second)
	if strings.Count(room, "-") != 2 {
		t.Fatalf("stdout should carry just the room code, got %q", room)
	}

	outDir := t.TempDir()
	recvCmd := clientCmd(bin, "-server", serverAddr, "-receive", room, "-out", outDir, "-yes")
	recvOut, err := recvCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("receive failed: %v\n%s", err, recvOut)
	}

	waitFor(t, sendCmd, 60*time.Second, &sendLog)

	for name, data := range want {
		got, err := os.ReadFile(filepath.Join(outDir, name))
		if err != nil {
			t.Errorf("%s was not saved: %v", name, err)
			continue
		}
		if !bytes.Equal(got, data) {
			t.Errorf("%s: content differs from the original", name)
		}
	}
}

// TestHeadlessWrongCodeFails checks that a mistyped code is a non-zero
// exit, not a silent success — the thing a script actually depends on.
func TestHeadlessWrongCodeFails(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a binary; skipped under -short")
	}

	bin := buildClient(t)
	_, _, serverAddr := newWSServer(t)

	cmd := clientCmd(bin, "-server", serverAddr, "-receive", "zebra-zebra-99",
		"-out", t.TempDir(), "-yes")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("a wrong code exited successfully:\n%s", out)
	}
}

// TestVersionFlag covers the first question anyone asks about a bug
// report, which the program previously had no way to answer.
func TestVersionFlag(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a binary; skipped under -short")
	}

	out, err := clientCmd(buildClient(t), "-version").CombinedOutput()
	if err != nil {
		t.Fatalf("-version failed: %v\n%s", err, out)
	}
	for _, want := range []string{"puresend", "commit:", "built:", "go:"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("-version output is missing %q:\n%s", want, out)
		}
	}
}

// coverDirEnv names the directory `make cover` collects the client
// binary's coverage in. It cannot simply be GOCOVERDIR: go test hands each
// test binary a GOCOVERDIR of its own, and a child that inherited that one
// would write its counters where nothing collects them.
const coverDirEnv = "PURESEND_COVERDIR"

// clientCmd runs the client binary, pointing its coverage output at
// coverDirEnv when there is one.
func clientCmd(bin string, args ...string) *exec.Cmd {
	cmd := exec.Command(bin, args...)
	if dir := os.Getenv(coverDirEnv); dir != "" {
		cmd.Env = append(os.Environ(), "GOCOVERDIR="+dir)
	}
	return cmd
}

// buildClient compiles the client once per test binary and returns its path.
func buildClient(t *testing.T) string {
	t.Helper()
	clientOnce.Do(func() {
		dir, err := os.MkdirTemp("", "ft-client")
		if err != nil {
			clientErr = err
			return
		}
		path := filepath.Join(dir, "puresend")
		args := []string{"build", "-o", path}
		// Under `make cover` the binary is built with coverage on, so the
		// code only a real process runs — flags, headless mode — counts
		// like everything the in-process tests reach.
		if os.Getenv(coverDirEnv) != "" {
			args = append(args, "-cover", "-covermode=atomic", "-coverpkg=puresend/...")
		}
		cmd := exec.Command("go", append(args, "puresend/cmd/client")...)
		if out, err := cmd.CombinedOutput(); err != nil {
			clientErr = fmt.Errorf("go build: %w\n%s", err, out)
			return
		}
		clientPath = path
	})
	if clientErr != nil {
		t.Fatal(clientErr)
	}
	return clientPath
}

// readLine reads one line, failing the test if it does not arrive in time.
func readLine(t *testing.T, r io.Reader, timeout time.Duration) string {
	t.Helper()
	type result struct {
		line string
		err  error
	}
	out := make(chan result, 1)
	go func() {
		var buf []byte
		one := make([]byte, 1)
		for {
			n, err := r.Read(one)
			if n > 0 {
				if one[0] == '\n' {
					out <- result{strings.TrimSpace(string(buf)), nil}
					return
				}
				buf = append(buf, one[0])
			}
			if err != nil {
				out <- result{"", err}
				return
			}
		}
	}()
	select {
	case res := <-out:
		if res.err != nil {
			t.Fatalf("could not read the room code: %v", res.err)
		}
		return res.line
	case <-time.After(timeout):
		t.Fatal("the sender never printed a room code")
		return ""
	}
}

// waitFor waits for a command to exit, reporting its log on failure.
//
// The log is only read after Wait returns: until then the buffer belongs
// to the goroutine os/exec uses to copy the child's stderr, and reading it
// alongside is a data race — one the race detector duly caught.
func waitFor(t *testing.T, cmd *exec.Cmd, timeout time.Duration, log *bytes.Buffer) {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	var err error
	select {
	case err = <-done:
	case <-time.After(timeout):
		cmd.Process.Kill()
		<-done
		t.Fatalf("the sender did not finish\n%s", log.String())
	}
	if err != nil {
		t.Fatalf("the sender exited with %v\n%s", err, log.String())
	}
}

// TestHeadlessSenderSurvivesAFailedAttempt: the interface keeps the room
// open when an attempt fails, and so must a script. The headless sender
// used to exit at the first failed attempt, taking the code with it, so a
// single dropped connection killed the whole job.
func TestHeadlessSenderSurvivesAFailedAttempt(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a binary; skipped under -short")
	}

	bin := buildClient(t)
	_, _, serverAddr := newWSServer(t)

	src := filepath.Join(t.TempDir(), "belge.txt")
	want := []byte("ikinci denemede gelmeli")
	if err := os.WriteFile(src, want, 0o644); err != nil {
		t.Fatal(err)
	}

	sendCmd := clientCmd(bin, "-server", serverAddr, "-send", src)
	stdout, err := sendCmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var sendLog bytes.Buffer
	sendCmd.Stderr = &sendLog
	if err := sendCmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sendCmd.Process.Kill() })
	room := readLine(t, stdout, 60*time.Second)

	sendExited := make(chan error, 1)
	go func() { sendExited <- sendCmd.Wait() }()

	// The first receiver says no.
	decline := clientCmd(bin, "-server", serverAddr, "-receive", room, "-out", t.TempDir())
	decline.Stdin = strings.NewReader("n\n")
	if out, err := decline.CombinedOutput(); err == nil {
		t.Fatalf("declining exited successfully:\n%s", out)
	}
	select {
	case err := <-sendExited:
		t.Fatalf("the sender gave up after one failed attempt (%v)\n%s", err, sendLog.String())
	case <-time.After(2 * time.Second):
	}

	// The second one, with the same code, gets the file.
	outDir := t.TempDir()
	accept := clientCmd(bin, "-server", serverAddr, "-receive", room, "-out", outDir, "-yes")
	if out, err := accept.CombinedOutput(); err != nil {
		t.Fatalf("the retry failed: %v\n%s", err, out)
	}
	select {
	case err := <-sendExited:
		if err != nil {
			t.Fatalf("the sender exited with %v\n%s", err, sendLog.String())
		}
	case <-time.After(60 * time.Second):
		t.Fatal("the sender did not finish after the successful attempt")
	}
	if got, _ := os.ReadFile(filepath.Join(outDir, "belge.txt")); !bytes.Equal(got, want) {
		t.Error("the file did not arrive intact")
	}
}

// TestHeadlessRejectsAMalformedCode: a code that cannot exist fails on
// the spot, before the network is even started.
func TestHeadlessRejectsAMalformedCode(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a binary; skipped under -short")
	}

	dead := "/ip4/127.0.0.1/tcp/1/ws/p2p/12D3KooWKKqpYTw3D8arNmcNG7ZK1mPfSH2cQ7ohZqHBmYN6eEAn"
	start := time.Now()
	out, err := clientCmd(buildClient(t), "-server", dead, "-receive", "kiraz", "-out", t.TempDir()).CombinedOutput()
	if err == nil {
		t.Fatalf("a malformed code exited successfully:\n%s", out)
	}
	if !strings.Contains(string(out), "not a room code") {
		t.Errorf("the error does not say what is wrong:\n%s", out)
	}
	if time.Since(start) > 5*time.Second {
		t.Errorf("rejecting a malformed code took %s; it should not touch the network", time.Since(start))
	}
}
