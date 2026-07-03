package main

import (
	"bytes"
	"strings"
	"testing"
)

func executeRootForTest(args ...string) (string, error) {
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	rootCmd.SetArgs(args)
	err := rootCmd.Execute()
	rootCmd.SetArgs(nil)
	rootCmd.SetOut(nil)
	rootCmd.SetErr(nil)
	return out.String(), err
}

func TestRootCommandIsSaaSOnly(t *testing.T) {
	_, err := executeRootForTest()
	if err == nil {
		t.Fatal("expected root command to reject local standalone mode")
	}
	if !strings.Contains(err.Error(), "local standalone mode has been removed") {
		t.Fatalf("error = %q, want SaaS-only message", err.Error())
	}
}

func TestHelpShowsOnlySaaSCommands(t *testing.T) {
	out, err := executeRootForTest("help")
	if err != nil {
		t.Fatalf("help returned error: %v", err)
	}

	for _, want := range []string{"saas-api", "version"} {
		if !strings.Contains(out, want) {
			t.Fatalf("help output missing %q:\n%s", want, out)
		}
	}
	for _, oldCommand := range []string{"chat", "setup", "gateway", "batch", "cron"} {
		if strings.Contains(out, oldCommand) {
			t.Fatalf("help output still exposes %q:\n%s", oldCommand, out)
		}
	}
}
