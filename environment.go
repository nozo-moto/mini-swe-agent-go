package main

import (
	"context"
	"errors"
	"os/exec"
	"syscall"
	"time"
)

type Model interface {
	Execute(ctx context.Context, dir, command string) (string, int, error) 
}

type BashModel struct {}
func (b BashModel) Execute(ctx context.Context, dir, command string) (string, int, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	cmd.Dir = dir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = time.Second
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return string(output), -1, ctx.Err()
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return string(output), exit.ExitCode(), nil
	}
	return string(output), 0, err
}
