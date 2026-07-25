// main is the entry point for the OpenDeploy CLI tool.
//
// The CLI connects to the Core API and provides a command-line interface
// for managing OpenDeploy without a browser. It is useful for scripting,
// CI/CD pipelines, and server administration.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

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
			updateScript := "curl -fsSL https://raw.githubusercontent.com/anrted/opendeploy/main/install.sh | bash"
			
			// Check if this is a dev update request
			reqFile := "/var/lib/opendeploy/update.request"
			if b, err := os.ReadFile(reqFile); err == nil {
				if strings.Contains(string(b), "dev") {
					fmt.Println("Downloading and applying the DEV OpenDeploy update...")
					updateScript = "curl -fsSL https://raw.githubusercontent.com/anrted/opendeploy/main/install.sh | bash -s -- --dev"
				} else {
					fmt.Println("Downloading and applying the latest OpenDeploy update...")
				}
				os.Remove(reqFile) // Clear the request file after reading
			} else {
				fmt.Println("Downloading and applying the latest OpenDeploy update...")
			}

			cmd := exec.Command("sh", "-c", updateScript)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				log.Fatalf("Update failed: %v", err)
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
