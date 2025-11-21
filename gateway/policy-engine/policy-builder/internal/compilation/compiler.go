package compilation

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/policy-engine/policy-builder/pkg/errors"
	"github.com/policy-engine/policy-builder/pkg/types"
)

// CompileBinary compiles the policy engine binary with all discovered policies
func CompileBinary(srcDir string, options *types.CompilationOptions) error {
	fmt.Println("Starting compilation phase...")

	// Step 1: go mod download
	if err := runGoModDownload(srcDir); err != nil {
		return errors.NewCompilationError("go mod download failed", err)
	}

	// Step 2: go mod tidy
	if err := runGoModTidy(srcDir); err != nil {
		return errors.NewCompilationError("go mod tidy failed", err)
	}

	// Step 3: Compile binary
	if err := runGoBuild(srcDir, options); err != nil {
		return errors.NewCompilationError("go build failed", err)
	}

	// Step 4: Optional UPX compression
	if options.EnableUPX {
		if err := runUPXCompression(options.OutputPath); err != nil {
			// UPX failure is non-fatal
			fmt.Printf("Warning: UPX compression failed: %v\n", err)
		}
	}

	fmt.Printf("✓ Binary compiled successfully: %s\n", options.OutputPath)
	return nil
}

// runGoModDownload downloads module dependencies
func runGoModDownload(srcDir string) error {
	fmt.Println("  → go mod download")

	// Debug: log go.mod path
	goModPath := filepath.Join(srcDir, "go.mod")
	slog.Debug("go mod download", "srcDir", srcDir, "goModPath", goModPath)

	cmd := exec.Command("go", "mod", "download")
	cmd.Dir = srcDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		// On error, print the go.mod file for debugging
		printGoModForDebug(goModPath)
		return fmt.Errorf("go mod download failed: %w", err)
	}

	return nil
}

// runGoModTidy tidies module dependencies
func runGoModTidy(srcDir string) error {
	fmt.Println("  → go mod tidy")

	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = srcDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go mod tidy failed: %w", err)
	}

	return nil
}

// runGoBuild compiles the Go binary
func runGoBuild(srcDir string, options *types.CompilationOptions) error {
	fmt.Println("  → go build (static binary)")

	// Ensure output directory exists
	outputDir := filepath.Dir(options.OutputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Build arguments
	args := []string{"build"}

	// Add build tags
	if len(options.BuildTags) > 0 {
		tags := ""
		for i, tag := range options.BuildTags {
			if i > 0 {
				tags += ","
			}
			tags += tag
		}
		args = append(args, "-tags", tags)
	}

	// Add ldflags
	if options.LDFlags != "" {
		args = append(args, "-ldflags", options.LDFlags)
	}

	// Add output path
	args = append(args, "-o", options.OutputPath)

	// Add main package (cmd/policy-engine)
	args = append(args, "./cmd/policy-engine")

	// Create command
	cmd := exec.Command("go", args...)
	cmd.Dir = srcDir

	// Set environment for static binary
	cmd.Env = os.Environ()
	if !options.CGOEnabled {
		cmd.Env = append(cmd.Env, "CGO_ENABLED=0")
	}
	if options.TargetOS != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("GOOS=%s", options.TargetOS))
	}
	if options.TargetArch != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("GOARCH=%s", options.TargetArch))
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go build failed: %w", err)
	}

	return nil
}

// runUPXCompression compresses the binary with UPX
func runUPXCompression(binaryPath string) error {
	fmt.Println("  → upx compression (optional)")

	// Check if UPX is available
	if _, err := exec.LookPath("upx"); err != nil {
		return fmt.Errorf("upx not found in PATH")
	}

	cmd := exec.Command("upx", "--best", "--lzma", binaryPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("upx compression failed: %w", err)
	}

	fmt.Println("  ✓ Binary compressed with UPX")
	return nil
}

// printGoModForDebug prints the go.mod file contents for debugging
func printGoModForDebug(goModPath string) {
	fmt.Println("\n========================================")
	fmt.Println("DEBUG: go.mod contents")
	fmt.Println("========================================")

	content, err := os.ReadFile(goModPath)
	if err != nil {
		slog.Error("failed to read go.mod for debug", "path", goModPath, "error", err)
		return
	}

	fmt.Println(string(content))
	fmt.Println("========================================\n")

	slog.Debug("go.mod file dumped for debugging", "path", goModPath, "size", len(content))
}
