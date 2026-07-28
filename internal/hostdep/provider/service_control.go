package provider

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

const defaultProviderCommandTimeout = 30 * time.Second

type providerCommandRunner func(context.Context, string, ...string) ([]byte, error)

func runProviderCommand(timeout time.Duration, runner providerCommandRunner, name string, args ...string) ([]byte, error) {
	if timeout <= 0 {
		timeout = defaultProviderCommandTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if runner == nil {
		runner = func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return exec.CommandContext(ctx, name, args...).CombinedOutput()
		}
	}
	output, err := runner(ctx, name, args...)
	if ctx.Err() != nil {
		return output, fmt.Errorf("command timed out after %s: %w", timeout, ctx.Err())
	}
	return output, err
}

func controlSystemdService(service, action string, timeout time.Duration, runner providerCommandRunner) error {
	output, err := runProviderCommand(timeout, runner, "systemctl", action, service)
	if err != nil {
		return fmt.Errorf("systemctl %s %s failed: %w\n%s", action, service, err, string(output))
	}
	return nil
}
