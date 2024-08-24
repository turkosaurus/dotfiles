package main

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestShellcheck(t *testing.T) {
	cmd := exec.Command("sh", "-c", "tests/shellcheck")
	t.Logf("command: %s\n", cmd.String())
	output, err := cmd.Output()
	t.Logf("output:\n%s\n", output)
	require.NoError(t, err)

	cmd = exec.Command("sh", "-c", "shellcheck bin/*/*")
	t.Logf("command: %s\n", cmd.String())
	output, err = cmd.Output()
	require.Empty(t, string(output))
	require.NoError(t, err)

	cmd = exec.Command("sh", "-c", "../..bin/test")
	t.Logf("command: %s\n", cmd.String())
	output, err = cmd.Output()
	require.Empty(t, string(output))
	require.NoError(t, err)
}
