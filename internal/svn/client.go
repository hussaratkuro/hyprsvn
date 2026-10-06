package svn

import (
	"bufio"
	"os"
	"os/exec"
	"strings"

	"github.com/hussaratkuro/hyprsvn/internal/model"
)

// Run executes an svn command and returns combined output.
func Run(r model.Repo, args ...string) (string, error) {
	finalArgs := baseArgs(r, true)
	finalArgs = append(finalArgs, args...)

	cmd := exec.Command("svn", finalArgs...)
	cmd.Dir = r.Path
	withPassword(cmd, r)

	out, err := cmd.CombinedOutput()
	return string(out), err
}

// withPassword feeds the repository password to svn's --password-from-stdin.
func withPassword(cmd *exec.Cmd, r model.Repo) {
	if r.Password != "" {
		cmd.Stdin = strings.NewReader(r.Password + "\n")
	}
}

// Cat returns repository file bytes without mixing command diagnostics into
// the content. Arguments follow `svn cat`, for example "-r", "BASE", path.
func Cat(r model.Repo, args ...string) ([]byte, error) {
	finalArgs := baseArgs(r, true)
	finalArgs = append(finalArgs, "cat")
	finalArgs = append(finalArgs, args...)
	cmd := exec.Command("svn", finalArgs...)
	cmd.Dir = r.Path
	withPassword(cmd, r)
	return cmd.Output()
}

// StreamLines runs an svn command and calls emit once per output line.
func StreamLines(r model.Repo, emit func(string), args ...string) error {
	finalArgs := baseArgs(r, true)
	finalArgs = append(finalArgs, args...)

	cmd := exec.Command("svn", finalArgs...)
	cmd.Dir = r.Path
	withPassword(cmd, r)

	pr, pw, err := os.Pipe()
	if err != nil {
		return err
	}
	cmd.Stdout = pw
	cmd.Stderr = pw

	if err := cmd.Start(); err != nil {
		pw.Close()
		pr.Close()
		return err
	}

	pw.Close()

	scanner := bufio.NewScanner(pr)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		emit(scanner.Text())
	}
	pr.Close()

	return cmd.Wait()
}

// Interactive runs an svn command with full terminal I/O attached.
func Interactive(r model.Repo, args ...string) error {
	finalArgs := baseArgs(r, false)
	for i, a := range finalArgs {
		if a == "--password-from-stdin" { // stdin is the terminal here
			finalArgs = append(finalArgs[:i], append([]string{"--password", r.Password}, finalArgs[i+1:]...)...)
			break
		}
	}
	finalArgs = append(finalArgs, args...)

	cmd := exec.Command("svn", finalArgs...)
	cmd.Dir = r.Path
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// WorkingCopyVersion runs svnversion and returns its output.
func WorkingCopyVersion(r model.Repo) (string, error) {
	cmd := exec.Command("svnversion", ".")
	cmd.Dir = r.Path
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func baseArgs(r model.Repo, nonInteractive bool) []string {
	var args []string
	if r.Username != "" {
		args = append(args, "--username", r.Username)
	}
	if r.Password != "" {
		// The password goes through stdin (withPassword), never through the
		// command line where other local users could read it in ps.
		args = append(args, "--password-from-stdin")
		if nonInteractive {
			args = append(args, "--non-interactive")
		}
		args = append(args, "--no-auth-cache")
	}
	return args
}
