//go:build !windows

/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package gateway

import (
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestProcessAliveForAFinishedProcess(t *testing.T) {
	done := exec.Command("true")
	if err := done.Run(); err != nil {
		t.Fatal(err)
	}
	if processAlive(done.Process.Pid) {
		t.Fatalf("processAlive(%d) = true for a process that has exited", done.Process.Pid)
	}
}

// Closing the terminal sends SIGHUP; it must reach docker instead of ending ap
// before the credentials file is deleted.
func TestForwardSignalsPassesHangupToTheChild(t *testing.T) {
	child := exec.Command("sleep", "30")
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	stop := forwardSignals(child.Process)
	defer stop()
	exited := make(chan error, 1)
	go func() { exited <- child.Wait() }()

	if err := syscall.Kill(os.Getpid(), syscall.SIGHUP); err != nil {
		t.Fatal(err)
	}

	select {
	case err := <-exited:
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("child ended with %v, want termination by SIGHUP", err)
		}
		status, ok := exitErr.Sys().(syscall.WaitStatus)
		if !ok || !status.Signaled() || status.Signal() != syscall.SIGHUP {
			t.Fatalf("child ended with %v, want termination by SIGHUP", err)
		}
	case <-time.After(10 * time.Second):
		_ = child.Process.Kill()
		t.Fatal("child did not receive SIGHUP within 10s")
	}
}

// buildah from and inspect create and read the working container that the
// deferred buildah rm removes, so they must forward signals too and report their stdout.
func TestRunForwardingCapturesStdoutAndForwardsSignals(t *testing.T) {
	logFile, err := os.CreateTemp(t.TempDir(), "log")
	if err != nil {
		t.Fatal(err)
	}
	defer logFile.Close()

	type result struct {
		out string
		err error
	}
	done := make(chan result, 1)
	go func() {
		out, err := runForwarding("sh", []string{"-c", trapTermScript}, builderAccessRun{}, logFile, true)
		done <- result{out, err}
	}()

	r := terminateWhenReady(t, logFile.Name(), done)
	if r.err != nil || r.out != "stopped\n" {
		t.Fatalf("runForwarding() = %q, %v; want the child's stdout after it received SIGTERM", r.out, r.err)
	}
}

// buildah rm must finish, and runs before the credentials file is deleted, so it forwards signals too.
func TestRunQuietForwardsSignals(t *testing.T) {
	logFile, err := os.CreateTemp(t.TempDir(), "log")
	if err != nil {
		t.Fatal(err)
	}
	defer logFile.Close()

	done := make(chan struct{}, 1)
	go func() {
		runQuiet(logFile, true, "sh", "-c", trapTermScript)
		done <- struct{}{}
	}()

	terminateWhenReady(t, logFile.Name(), done)
	if b, _ := os.ReadFile(logFile.Name()); !strings.Contains(string(b), "stopped") {
		t.Fatalf("log = %q, want the child to report SIGTERM", b)
	}
}

// trapTermScript prints "ready" to stderr once its SIGTERM trap is installed, and
// "stopped" to stdout when the trap runs. The trap ignores further SIGTERMs (the
// test may send several), and the loop ends on its own so a failed test leaves
// no process behind.
const trapTermScript = `trap 'trap "" TERM; echo stopped; exit 0' TERM; echo ready >&2; ` +
	`i=0; while [ $i -lt 300 ]; do sleep 0.1; i=$((i+1)); done; exit 1`

// terminateWhenReady waits for "ready" in the log file, then sends SIGTERM to
// the test process until done delivers, and returns what it delivered.
func terminateWhenReady[T any](t *testing.T, logPath string, done <-chan T) T {
	t.Helper()
	// Keep SIGTERM from ending the test process if it arrives before
	// forwarding is registered.
	guard := make(chan os.Signal, 1)
	signal.Notify(guard, syscall.SIGTERM)
	defer signal.Stop(guard)

	deadline := time.After(10 * time.Second)
	for {
		if b, _ := os.ReadFile(logPath); strings.Contains(string(b), "ready") {
			break
		}
		select {
		case <-deadline:
			t.Fatal("child did not install its trap within 10s")
		case <-time.After(20 * time.Millisecond):
		}
	}

	// Forwarding is registered right after the child starts; repeat the signal
	// until the child reports it, in case the first one came before that.
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for {
		if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
			t.Fatal(err)
		}
		select {
		case v := <-done:
			return v
		case <-deadline:
			t.Fatal("child did not receive SIGTERM within 10s")
		case <-tick.C:
		}
	}
}
