// main is the entry point for the OpenDeploy CLI tool.
//
// The CLI connects to the Core API and provides a command-line interface
// for managing OpenDeploy without a browser. It is useful for scripting,
// CI/CD pipelines, and server administration.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/anrted/opendeploy/pkg/version"
)

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(version.Info())
		os.Exit(0)
	}

	args := flag.Args()
	if len(args) == 0 {
		printUsage()
		os.Exit(1)
	}

	switch args[0] {
	case "version":
		fmt.Println(version.Info())
	case "status":
		fmt.Fprintln(os.Stderr, "error: CLI API client not yet implemented (Stage 8)")
		os.Exit(1)
	case "update":
		if len(args) > 1 && args[1] == "--apply" {
			dev := false
			// Check if this is a dev update request
			reqFile := "/var/lib/opendeploy/update.request"
			if b, err := os.ReadFile(reqFile); err == nil {
				request := strings.TrimSpace(string(b))
				if request == "dev" {
					fmt.Println("Downloading and applying the DEV OpenDeploy update...")
					dev = true
				} else if _, err := time.Parse(time.RFC3339, request); err != nil {
					log.Fatalf("Update request is malformed")
				} else {
					fmt.Println("Downloading and applying the latest OpenDeploy update...")
				}
			} else {
				fmt.Println("Downloading and applying the latest OpenDeploy update...")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
			defer cancel()
			if err := applyUpdate(ctx, dev); err != nil {
				log.Fatalf("Update failed: %v", err)
			}
			if err := os.Remove(reqFile); err != nil && !os.IsNotExist(err) {
				log.Printf("warning: update succeeded but request cleanup failed: %v", err)
			}
			fmt.Println("Update completed successfully.")
			os.Exit(0)
		}
		fmt.Println("Run 'opendeploy update --apply' to apply the latest update.")
	default:
		log.Printf("unknown command: %q\n", args[0])
		printUsage()
		os.Exit(1)
	}
}

func applyUpdate(ctx context.Context, dev bool) error {
	const installerURL = "https://raw.githubusercontent.com/anrted/opendeploy/main/install.sh"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, installerURL, nil)
	if err != nil {
		return fmt.Errorf("create installer request: %w", err)
	}
	client := &http.Client{Timeout: 2 * time.Minute}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("download installer: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("download installer: HTTP %d", response.StatusCode)
	}
	file, err := os.CreateTemp("", "opendeploy-installer-*.sh")
	if err != nil {
		return fmt.Errorf("create installer file: %w", err)
	}
	path := file.Name()
	defer os.Remove(path)
	if err := file.Chmod(0o700); err != nil {
		_ = file.Close()
		return fmt.Errorf("secure installer file: %w", err)
	}
	written, copyErr := io.Copy(file, io.LimitReader(response.Body, (1<<20)+1))
	closeErr := file.Close()
	if copyErr != nil {
		return fmt.Errorf("write installer: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close installer: %w", closeErr)
	}
	if written > 1<<20 {
		return fmt.Errorf("installer exceeds size limit")
	}
	arguments := []string{path}
	if dev {
		arguments = append(arguments, "--dev")
	}
	command := exec.CommandContext(ctx, "/bin/sh", arguments...)
	command.Env = []string{"PATH=/usr/sbin:/usr/bin:/sbin:/bin", "HOME=/root", "LANG=C.UTF-8", "LC_ALL=C.UTF-8"}
	command.Stdout, command.Stderr = os.Stdout, os.Stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("installer failed: %w", err)
	}
	return nil
}

func printUsage() {
	fmt.Println(`OpenDeploy CLI

Usage: opendeploy <command> [flags]

Commands:
  version     Print version information
  status      Show system status
  modules     Manage modules
  sites       Manage sites
  services    Manage services

Run 'opendeploy <command> --help' for more information on a command.`)
}
