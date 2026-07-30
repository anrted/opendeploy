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
	"github.com/anrted/opendeploy/internal/core/auth"
	"github.com/anrted/opendeploy/internal/platform/database/sqlite"
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
	case "admin":
		runAdmin(args[1:])
	default:
		log.Printf("unknown command: %q\n", args[0])
		printUsage()
		os.Exit(1)
	}
}

func runAdmin(args []string) {
	if !isRoot() {
		log.Fatal("OpenDeploy administrator recovery must run as root")
	}
	flags := flag.NewFlagSet("admin reset-password", flag.ExitOnError)
	username := flags.String("username", "admin", "user whose password will be regenerated")
	databasePath := flags.String("database", "/var/lib/opendeploy/data.db", "OpenDeploy SQLite database path")
	if len(args) == 0 || args[0] != "reset-password" {
		fmt.Println("Usage: opendeploy admin reset-password [--username admin] [--database /var/lib/opendeploy/data.db]")
		return
	}
	if err := flags.Parse(args[1:]); err != nil {
		log.Fatal(err)
	}
	terminal, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	if err != nil {
		log.Fatalf("A controlling terminal is required to display the generated password securely: %v", err)
	}
	defer func() { _ = terminal.Close() }()
	db, err := sqlite.Open(*databasePath)
	if err != nil {
		log.Fatalf("Open database: %v", err)
	}
	defer func() { _ = db.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	password, err := auth.ResetGeneratedPassword(
		ctx,
		auth.NewSQLiteUserRepository(db.DB),
		auth.NewSQLiteSessionRepository(db.DB),
		*username,
	)
	if err != nil {
		log.Fatalf("Reset password: %v", err)
	}
	if err := writePasswordToTerminal(terminal, *username, password); err != nil {
		log.Fatalf("Password was reset, but the new credential could not be shown securely: %v", err)
	}
	fmt.Println("All existing sessions for this user have been revoked.")
}

func writePasswordToTerminal(terminal *os.File, username, password string) error {
	_, err := terminal.WriteString(
		"OpenDeploy password regenerated successfully.\nUsername: " + username +
			"\nPassword: " + password + "\n",
	)
	return err
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
		removeRequest := func() {
			if err := os.Remove(requestFile); err != nil && !os.IsNotExist(err) {
				log.Printf("warning: request cleanup failed: %v", err)
			}
		}
		if request.Operation == "rollback" {
			entry, err := engine.Rollback(ctx, request.TransactionID)
			if err != nil {
				removeRequest()
				log.Fatalf("Rollback failed: %v", err)
			}
			removeRequest()
			fmt.Printf("Rollback %s completed successfully.\n", entry.ID)
			return
		}
		backupEngine := systembackup.NewEngine(systembackup.DefaultConfig(), version.Version)
		backupEngine.Runtime = systembackup.SystemRuntime{}
		if request.Operation == "backup" {
			manifest, path, err := backupEngine.Create(ctx, request.Reason)
			if err != nil {
				removeRequest()
				log.Fatalf("Backup failed: %v", err)
			}
			removeRequest()
			fmt.Printf("Backup %s created: %s\n", manifest.ID, path)
			return
		}
		if request.Operation == "restore" {
			archivePath := filepath.Join(systembackup.DefaultConfig().BackupDir, request.Archive)
			manifest, err := backupEngine.Restore(ctx, archivePath)
			if err != nil {
				removeRequest()
				log.Fatalf("Restore failed: %v", err)
			}
			removeRequest()
			fmt.Printf("Backup %s restored successfully.\n", manifest.ID)
			return
		}
		fmt.Printf("Applying signed OpenDeploy release %s...\n", request.Tag)
		entry, err := engine.Apply(ctx, request.Tag, version.Version)
		if err != nil {
			removeRequest()
			log.Fatalf("Update failed (transaction %s): %v", entry.ID, err)
		}
		removeRequest()
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
  admin       Recover administrator access

Run 'opendeploy <command> --help' for more information on a command.`)
}
