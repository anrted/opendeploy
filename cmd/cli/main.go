// main is the entry point for the OpenDeploy CLI tool.
//
// The CLI connects to the Core API and provides a command-line interface
// for managing OpenDeploy without a browser. It is useful for scripting,
// CI/CD pipelines, and server administration.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	systembackup "github.com/anrted/opendeploy/internal/backup"
	secureupdate "github.com/anrted/opendeploy/internal/update"
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
		runUpdate(args[1:])
	case "backup":
		runBackup(args[1:])
	default:
		log.Printf("unknown command: %q\n", args[0])
		printUsage()
		os.Exit(1)
	}
}

func runUpdate(args []string) {
	if !isRoot() {
		log.Fatal("OpenDeploy updater must run as root")
	}
	config := secureupdate.DefaultConfig()
	var verifier secureupdate.SignatureVerifier = secureupdate.DefaultSigstoreVerifier()
	if cosignPath := os.Getenv("OD_UPDATE_COSIGN_PATH"); cosignPath != "" {
		verifier.(*secureupdate.SigstoreVerifier).CosignPath = cosignPath
	}
	if keyring := os.Getenv("OD_UPDATE_GPG_KEYRING"); keyring != "" {
		config.Signature = "release-manifest.json.asc"
		verifier = &secureupdate.GPGVerifier{Keyring: keyring}
	}
	engine := secureupdate.NewEngine(config, nil, verifier, &secureupdate.SystemRuntime{})
	engine.Backup = systembackup.NewEngine(systembackup.DefaultConfig(), version.Version)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	if len(args) == 1 && args[0] == "--apply" {
		const requestFile = "/var/lib/opendeploy/update.request"
		data, err := os.ReadFile(requestFile)
		if err != nil {
			log.Fatalf("Read update request: %v", err)
		}
		var request secureupdate.UpdateRequest
		if err := json.Unmarshal(data, &request); err != nil || request.Validate() != nil {
			log.Fatal("Update request is malformed")
		}
		if request.Operation == "rollback" {
			entry, err := engine.Rollback(ctx, request.TransactionID)
			if err != nil {
				log.Fatalf("Rollback failed: %v", err)
			}
			if err := os.Remove(requestFile); err != nil && !os.IsNotExist(err) {
				log.Printf("warning: rollback succeeded but request cleanup failed: %v", err)
			}
			fmt.Printf("Rollback %s completed successfully.\n", entry.ID)
			return
		}
		backupEngine := systembackup.NewEngine(systembackup.DefaultConfig(), version.Version)
		backupEngine.Runtime = systembackup.SystemRuntime{}
		if request.Operation == "backup" {
			manifest, path, err := backupEngine.Create(ctx, request.Reason)
			if err != nil {
				log.Fatalf("Backup failed: %v", err)
			}
			if err := os.Remove(requestFile); err != nil && !os.IsNotExist(err) {
				log.Printf("warning: backup succeeded but request cleanup failed: %v", err)
			}
			fmt.Printf("Backup %s created: %s\n", manifest.ID, path)
			return
		}
		if request.Operation == "restore" {
			archivePath := filepath.Join(systembackup.DefaultConfig().BackupDir, request.Archive)
			manifest, err := backupEngine.Restore(ctx, archivePath)
			if err != nil {
				log.Fatalf("Restore failed: %v", err)
			}
			if err := os.Remove(requestFile); err != nil && !os.IsNotExist(err) {
				log.Printf("warning: restore succeeded but request cleanup failed: %v", err)
			}
			fmt.Printf("Backup %s restored successfully.\n", manifest.ID)
			return
		}
		fmt.Printf("Applying signed OpenDeploy release %s...\n", request.Tag)
		entry, err := engine.Apply(ctx, request.Tag, version.Version)
		if err != nil {
			log.Fatalf("Update failed (transaction %s): %v", entry.ID, err)
		}
		if err := os.Remove(requestFile); err != nil && !os.IsNotExist(err) {
			log.Printf("warning: update succeeded but request cleanup failed: %v", err)
		}
		fmt.Printf("Update %s completed successfully.\n", entry.ID)
		return
	}
	if len(args) >= 1 && args[0] == "rollback" {
		transactionID := ""
		if len(args) > 1 {
			transactionID = args[1]
		}
		entry, err := engine.Rollback(ctx, transactionID)
		if err != nil {
			log.Fatalf("Rollback failed: %v", err)
		}
		fmt.Printf("Rollback %s completed successfully.\n", entry.ID)
		return
	}
	if len(args) == 1 && args[0] == "history" {
		entries, err := engine.History()
		if err != nil {
			log.Fatalf("Read update history: %v", err)
		}
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		_ = encoder.Encode(entries)
		return
	}
	fmt.Println("Usage: opendeploy update --apply | rollback [transaction-id] | history")
}

func runBackup(args []string) {
	if !isRoot() {
		log.Fatal("OpenDeploy backup operations must run as root")
	}
	engine := systembackup.NewEngine(systembackup.DefaultConfig(), version.Version)
	engine.Runtime = systembackup.SystemRuntime{}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Hour)
	defer cancel()
	if len(args) == 0 || args[0] == "create" {
		reason := "manual"
		if len(args) > 1 {
			reason = args[1]
		}
		manifest, path, err := engine.Create(ctx, reason)
		if err != nil {
			log.Fatalf("Backup failed: %v", err)
		}
		fmt.Printf("Backup %s created: %s\n", manifest.ID, path)
		return
	}
	if len(args) == 2 && args[0] == "verify" {
		manifest, err := engine.Verify(ctx, args[1])
		if err != nil {
			log.Fatalf("Backup verification failed: %v", err)
		}
		fmt.Printf("Backup %s is valid (%d files, %d bytes).\n", manifest.ID, len(manifest.Entries), manifest.TotalBytes)
		return
	}
	if len(args) == 2 && args[0] == "restore" {
		manifest, err := engine.Restore(ctx, args[1])
		if err != nil {
			log.Fatalf("Restore failed: %v", err)
		}
		fmt.Printf("Backup %s restored successfully. Restart OpenDeploy and managed services.\n", manifest.ID)
		return
	}
	if len(args) == 1 && args[0] == "history" {
		operations, err := engine.History()
		if err != nil {
			log.Fatalf("Read backup history: %v", err)
		}
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		_ = encoder.Encode(operations)
		return
	}
	fmt.Println("Usage: opendeploy backup create [reason] | verify <archive> | restore <archive> | history")
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
  update      Apply, inspect or roll back signed releases
  backup      Create, verify or restore full system backups

Run 'opendeploy <command> --help' for more information on a command.`)
}
