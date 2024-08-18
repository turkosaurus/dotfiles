package main

import (
	"fmt"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMain(t *testing.T) {
	word := "hello world"
	cmd := exec.Command("echo", word)
	output, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("%s", output)
}

func TestSpellcheck(t *testing.T) {
	cmd := exec.Command("sh", "-c", "bin/tests/shellcheck")
	t.Logf("command: %s\n", cmd.String())
	output, err := cmd.Output()
	t.Logf("output:\n%s\n", output)
	require.NoError(t, err)

	cmd = exec.Command("sh", "-c", "shellcheck bin/*/*")
	t.Logf("command: %s\n", cmd.String())
	output, err = cmd.Output()
	require.Empty(t, string(output))
	require.NoError(t, err)
}
