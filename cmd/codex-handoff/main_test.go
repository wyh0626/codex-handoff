package main

import "testing"

func TestHasArg(t *testing.T) {
	args := []string{"export", "--project", "/tmp/project", "--redact"}
	if !hasArg(args, "--redact") {
		t.Fatal("hasArg did not find an existing flag")
	}
	if hasArg(args, "--include-archived") {
		t.Fatal("hasArg found a flag that was not present")
	}
}
