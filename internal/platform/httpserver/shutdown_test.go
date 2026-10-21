//go:build unix

package httpserver

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	if os.Getenv("BE_SHUTDOWN_SERVER") == "1" {
		if err := serveUntilSignal(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// serveUntilSignal is the test double of a process: a slow request, then SIGTERM.
// The route exists only in this test.
func serveUntilSignal() error {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	fmt.Printf("addr %s\n", ln.Addr().String())
	mux := http.NewServeMux()
	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("started"))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		time.Sleep(time.Second)
		_, _ = w.Write([]byte("done"))
	})
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM)
	defer stop()
	return Serve(ctx, ln, mux, 5*time.Second)
}

func TestSIGTERMFinishesInflightRequest(t *testing.T) {
	cmd := exec.Command(os.Args[0])
	cmd.Env = append(os.Environ(), "BE_SHUTDOWN_SERVER=1")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if cmd.ProcessState == nil && cmd.Process != nil {
			_ = cmd.Process.Kill()
			_, _ = cmd.Process.Wait()
		}
	})

	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	addr := strings.TrimPrefix(strings.TrimSpace(line), "addr ")

	resp, err := http.Get("http://" + addr + "/slow")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Error(err)
		}
	}()

	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK || string(body) != "starteddone" {
		t.Fatalf("response %d %q", resp.StatusCode, body)
	}

	wait := make(chan error, 1)
	go func() { wait <- cmd.Wait() }()
	select {
	case err := <-wait:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server still running after the in-flight request finished")
	}
}
