package desktop

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"
)

func TestCancelTreeHelper(t *testing.T) {
	dir := os.Getenv("NSF_CANCEL_TEST_DIR")
	if dir == "" {
		return
	}
	if os.Args[len(os.Args)-1] == "child" {
		if err := os.WriteFile(filepath.Join(dir, "child.pid"), []byte(strconv.Itoa(os.Getpid())), 0600); err != nil {
			os.Exit(91)
		}
		time.Sleep(time.Minute)
		os.Exit(0)
	}
	child := exec.Command(os.Args[0], "-test.run=^TestCancelTreeHelper$", "--", "child")
	quiet(child)
	if err := child.Run(); err != nil {
		os.Exit(92)
	}
	os.Exit(0)
}

func TestCancelTreeStopsChildHelpers(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestCancelTreeHelper$", "--", "parent")
	cmd.Env = append(os.Environ(), "NSF_CANCEL_TEST_DIR="+dir)
	quiet(cmd)
	cancelTree(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	var pid int
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if b, err := os.ReadFile(filepath.Join(dir, "child.pid")); err == nil {
			pid, _ = strconv.Atoi(string(b))
			if pid != 0 {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	if pid == 0 {
		cancel()
		<-done
		t.Fatal("child helper did not become ready")
	}
	// Hold a handle to this exact helper, so cleanup cannot target a reused PID.
	handle, err := syscall.OpenProcess(syscall.SYNCHRONIZE|syscall.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		cancel()
		<-done
		t.Fatal(err)
	}
	defer syscall.CloseHandle(handle)
	defer syscall.TerminateProcess(handle, 93)
	if event, err := syscall.WaitForSingleObject(handle, 0); err != nil || event != syscall.WAIT_TIMEOUT {
		t.Fatalf("child was not alive before cancellation: event=%d err=%v", event, err)
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled parent unexpectedly succeeded")
		}
	case <-time.After(12 * time.Second):
		t.Fatal("parent did not stop after cancellation")
	}
	if event, err := syscall.WaitForSingleObject(handle, 5000); err != nil || event != syscall.WAIT_OBJECT_0 {
		t.Fatalf("child helper survived cancellation: event=%d err=%v", event, err)
	}
}
