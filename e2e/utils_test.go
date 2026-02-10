package e2e

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/gomega"
)

const (
	kubectlPath   = "kubectl"
	kustomizePath = "kustomize"
	dockerPath    = "docker"
	yqPath        = "yq"
)

func runCommand(path string, input []byte, args ...string) ([]byte, []byte, error) {
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd := exec.Command(path, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if input != nil {
		cmd.Stdin = bytes.NewReader(input)
	}
	if err := cmd.Run(); err != nil {
		_, file := filepath.Split(path)
		return stdout.Bytes(), stderr.Bytes(), fmt.Errorf("%s failed with %s: stderr=%s", file, err, stderr)
	}
	return stdout.Bytes(), stderr.Bytes(), nil
}

func kubectl(input []byte, args ...string) ([]byte, []byte, error) {
	return runCommand(kubectlPath, input, args...)
}

func kubectlSafe(g Gomega, input []byte, args ...string) []byte {
	stdout, stderr, err := runCommand(kubectlPath, input, args...)
	g.Expect(err).NotTo(HaveOccurred(), "stdout: %s, stderr: %s", stdout, stderr)
	return stdout
}

func kustomizeSafe(g Gomega, input []byte, args ...string) []byte {
	stdout, stderr, err := runCommand(kustomizePath, input, args...)
	g.Expect(err).NotTo(HaveOccurred(), "stdout: %s, stderr: %s", stdout, stderr)
	return stdout
}

func yqSafe(g Gomega, input []byte, args ...string) []byte {
	stdout, stderr, err := runCommand(yqPath, input, args...)
	g.Expect(err).NotTo(HaveOccurred(), "input: %s, stdout: %s, stderr: %s", string(input), stdout, stderr)
	return stdout
}

func dockerSafe(g Gomega, input []byte, args ...string) []byte {
	stdout, stderr, err := runCommand(dockerPath, input, args...)
	g.Expect(err).NotTo(HaveOccurred(), "stdout: %s, stderr: %s", stdout, stderr)
	return stdout
}
