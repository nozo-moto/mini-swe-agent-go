package main

import (
	"context"
	"testing"
	"time"
)

func TestShellStateAndFiles(t *testing.T) {
	dir := t.TempDir()
	_, code, err := execute(context.Background(), dir, "export MINI_TEST_STATE=present; printf data > result.txt; cd /")
	if err != nil || code != 0 {
		t.Fatalf("exit=%d err=%v", code, err)
	}
	output, code, err := execute(context.Background(), dir, "printenv MINI_TEST_STATE; cat result.txt; printf stderr >&2")
	if err != nil || code != 0 || output != "datastderr" {
		t.Fatalf("output=%q exit=%d err=%v", output, code, err)
	}
}

func TestTimeoutKillsChildren(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, _, err := execute(ctx, dir, "(sleep 0.5; touch orphan) & wait")
	if err != context.DeadlineExceeded {
		t.Fatalf("expected timeout, got %v", err)
	}
	time.Sleep(600 * time.Millisecond)
	_, code, err := execute(context.Background(), dir, "test ! -e orphan")
	if err != nil || code != 0 {
		t.Fatal("child process survived timeout")
	}
}
