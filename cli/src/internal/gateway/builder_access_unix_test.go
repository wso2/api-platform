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
