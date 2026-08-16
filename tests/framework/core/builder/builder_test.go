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

package builder

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

type recordingRunner struct {
	mu       sync.Mutex
	commands []Command
	err      error
}

func (r *recordingRunner) Run(_ context.Context, command Command) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.commands = append(r.commands, command)
	return r.err
}

func TestBuildPlansAndRunsCommands(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, "platform-api"), 0o755))
	runner := &recordingRunner{}
	spec := Spec{
		Component:        "platform-api",
		SourceDir:        "platform-api",
		Images:           []Image{{Name: "platform-api:1.0.0", Dockerfile: "platform-api/Dockerfile", Context: "platform-api"}},
		SupportsCoverage: true,
		Plan: func(root, version string, coverage bool) ([]Command, error) {
			require.NotEmpty(t, root)
			require.Equal(t, "1.0.0", version)
			require.True(t, coverage)
			return []Command{{Directory: filepath.Join(root, "platform-api"), Args: []string{"make", "build"}}}, nil
		},
	}
	require.NoError(t, Build(context.Background(), spec, Request{
		RepoRoot: root, Version: "1.0.0", Coverage: true, Runner: runner,
	}))
	require.Len(t, runner.commands, 1)
}

func TestBuildRejectsUnsupportedCoverage(t *testing.T) {
	err := Build(context.Background(), Spec{
		Component: "api-portal",
		SourceDir: "api-portal",
		Images:    []Image{{Name: "api-portal:1.0.0", Dockerfile: "api-portal/Dockerfile", Context: "api-portal"}},
		Plan:      func(string, string, bool) ([]Command, error) { return nil, nil },
	}, Request{RepoRoot: t.TempDir(), Version: "1.0.0", Coverage: true, Runner: &recordingRunner{}})
	require.ErrorContains(t, err, "coverage instrumentation is not supported")
}

func TestBuildStopsAfterCommandFailure(t *testing.T) {
	runner := &recordingRunner{err: errors.New("docker failed")}
	spec := Spec{
		Component: "gateway",
		SourceDir: "gateway",
		Images:    []Image{{Name: "gateway:1.0.0", Dockerfile: "gateway/Dockerfile", Context: "gateway"}},
		Plan: func(root, _ string, _ bool) ([]Command, error) {
			return []Command{{Directory: root, Args: []string{"make", "build"}}, {Directory: root, Args: []string{"make", "second"}}}, nil
		},
	}
	err := Build(context.Background(), spec, Request{RepoRoot: t.TempDir(), Version: "1.0.0", Runner: runner})
	require.ErrorContains(t, err, "docker failed")
	require.Len(t, runner.commands, 1)
}

func TestBuildCanRunConcurrentlyWithIndependentRequests(t *testing.T) {
	runner := &recordingRunner{}
	spec := Spec{
		Component: "gateway",
		SourceDir: "gateway",
		Images:    []Image{{Name: "gateway:1.0.0", Dockerfile: "gateway/Dockerfile", Context: "gateway"}},
		Plan: func(root, version string, _ bool) ([]Command, error) {
			return []Command{{Directory: root, Args: []string{"make", version}}}, nil
		},
	}
	const runs = 20
	var wg sync.WaitGroup
	errs := make(chan error, runs)
	for i := 0; i < runs; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := Build(context.Background(), spec, Request{
				RepoRoot: t.TempDir(), Version: "current", Runner: runner,
			}); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	runner.mu.Lock()
	defer runner.mu.Unlock()
	require.Len(t, runner.commands, runs)
}

func TestBuildRejectsCommandOutsideRepository(t *testing.T) {
	root := t.TempDir()
	err := Build(context.Background(), Spec{
		Component: "gateway",
		SourceDir: "gateway",
		Images:    []Image{{Name: "gateway:1.0.0", Dockerfile: "gateway/Dockerfile", Context: "gateway"}},
		Plan: func(string, string, bool) ([]Command, error) {
			return []Command{{Directory: t.TempDir(), Args: []string{"make"}}}, nil
		},
	}, Request{RepoRoot: root, Version: "1.0.0", Runner: &recordingRunner{}})
	require.ErrorContains(t, err, "outside repository root")
}

func TestBuildManyPreservesOrderAndStopsOnFailure(t *testing.T) {
	root := t.TempDir()
	runner := &recordingRunner{}
	makeSpec := func(name string) Spec {
		return Spec{
			Component: name,
			SourceDir: name,
			Images:    []Image{{Name: name + ":test", Dockerfile: name + "/Dockerfile", Context: name}},
			Plan: func(repoRoot, _ string, _ bool) ([]Command, error) {
				return []Command{{Directory: repoRoot, Args: []string{"make", name}}}, nil
			},
		}
	}
	require.NoError(t, BuildMany(context.Background(), []Spec{makeSpec("one"), makeSpec("two")}, Request{
		RepoRoot: root, Version: "test", Runner: runner,
	}))
	require.Len(t, runner.commands, 2)
	require.Equal(t, []string{"one", "two"}, []string{runner.commands[0].Args[1], runner.commands[1].Args[1]})
}
