package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseCLI_NoArgsShowsHelp(t *testing.T) {
	opt, err := parseCLI([]string{"sub2api.exe"})
	if err != nil {
		t.Fatal(err)
	}
	if !opt.showHelp {
		t.Fatal("expected showHelp for no args")
	}
	if opt.runServer {
		t.Fatal("must not start server without args")
	}
}

func TestParseCLI_HelpFlags(t *testing.T) {
	for _, args := range [][]string{
		{"sub2api", "-h"},
		{"sub2api", "--help"},
		{"sub2api", "help"},
	} {
		opt, err := parseCLI(args)
		if err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if !opt.showHelp {
			t.Fatalf("%v: expected help", args)
		}
	}
}

func TestParseCLI_DIYFlags(t *testing.T) {
	opt, err := parseCLI([]string{
		"sub2api",
		"-deploy-mode=diy",
		"-auto-setup",
		"-admin-email=a@b.com",
		"-admin-password=secret",
		"-config=./config.yaml",
		"-port=8080",
	})
	if err != nil {
		t.Fatal(err)
	}
	if opt.showHelp || !opt.runServer {
		t.Fatalf("unexpected opt: %+v", opt)
	}
	if opt.deployMode != "diy" || opt.autoSetup != "true" {
		t.Fatalf("deploy/auto: %+v", opt)
	}
	if opt.adminEmail != "a@b.com" || opt.adminPassword != "secret" {
		t.Fatalf("admin: %+v", opt)
	}
	if opt.configFile != "./config.yaml" || opt.port != "8080" {
		t.Fatalf("config/port: %+v", opt)
	}
}

func TestParseCLI_ShortConfig(t *testing.T) {
	opt, err := parseCLI([]string{"sub2api", "-c", "my.yaml"})
	if err != nil {
		t.Fatal(err)
	}
	if opt.configFile != "my.yaml" {
		t.Fatalf("got %q", opt.configFile)
	}
}

func TestPrintUsageContainsKeyExamples(t *testing.T) {
	var buf bytes.Buffer
	printUsage(&buf)
	out := buf.String()
	for _, want := range []string{
		"-deploy-mode=diy",
		"-config",
		"-auto-setup",
		"-admin-email",
		"config.yaml",
		"优先级",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("usage missing %q", want)
		}
	}
}
