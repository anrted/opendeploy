package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"

	mysqlDriver "github.com/go-sql-driver/mysql"
)

// dbNameRegex ensures the database name only contains safe characters
var dbNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

// getDB opens a connection to the local MySQL server.
func (m *Module) getDB() (*sql.DB, error) {
	password := m.deps.Config.Get("root_password", "")
	if password == "" {
		return nil, fmt.Errorf("mysql administrator credentials are not configured")
	}
	driverConfig := mysqlDriver.NewConfig()
	driverConfig.User = "root"
	driverConfig.Passwd = password
	driverConfig.Net = "tcp"
	driverConfig.Addr = "127.0.0.1:3306"
	driverConfig.ParseTime = true
	db, err := sql.Open("mysql", driverConfig.FormatDSN())
	if err != nil {
		return nil, fmt.Errorf("failed to open mysql connection: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping mysql: %w", err)
	}
	return db, nil
}

// CreateDatabase securely creates a new database and a user with access to it.
func (m *Module) CreateDatabase(ctx context.Context, dbName, user, password string) error {
	if !dbNameRegex.MatchString(dbName) || !dbNameRegex.MatchString(user) {
		return fmt.Errorf("invalid database or username format")
	}

	db, err := m.getDB()
	if err != nil {
		return err
	}
	defer db.Close()

	// CREATE DATABASE and CREATE USER don't support prepared statements for identifiers.
	// So we rigorously validate using the regex above and then fmt.Sprintf.
	createDbSQL := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;", dbName)
	if _, err := db.ExecContext(ctx, createDbSQL); err != nil {
		return fmt.Errorf("failed to create database: %w", err)
	}

	createUserSQL := fmt.Sprintf("CREATE USER IF NOT EXISTS `%s`@'localhost' IDENTIFIED BY ?", user)
	if _, err := db.ExecContext(ctx, createUserSQL, password); err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	grantSQL := fmt.Sprintf("GRANT ALL PRIVILEGES ON `%s`.* TO `%s`@'localhost';", dbName, user)
	if _, err := db.ExecContext(ctx, grantSQL); err != nil {
		return fmt.Errorf("failed to grant privileges: %w", err)
	}

	flushSQL := "FLUSH PRIVILEGES;"
	if _, err := db.ExecContext(ctx, flushSQL); err != nil {
		return fmt.Errorf("failed to flush privileges: %w", err)
	}

	m.logger.Info("MySQL: Created database successfully", "db", dbName, "user", user)
	return nil
}

// DeleteDatabase securely removes a database and its associated user.
func (m *Module) DeleteDatabase(ctx context.Context, dbName, user string) error {
	if !dbNameRegex.MatchString(dbName) || !dbNameRegex.MatchString(user) {
		return fmt.Errorf("invalid database or username format")
	}

	db, err := m.getDB()
	if err != nil {
		return err
	}
	defer db.Close()

	dropDbSQL := fmt.Sprintf("DROP DATABASE IF EXISTS `%s`;", dbName)
	if _, err := db.ExecContext(ctx, dropDbSQL); err != nil {
		return fmt.Errorf("failed to drop database: %w", err)
	}

	dropUserSQL := fmt.Sprintf("DROP USER IF EXISTS `%s`@'localhost';", user)
	if _, err := db.ExecContext(ctx, dropUserSQL); err != nil {
		return fmt.Errorf("failed to drop user: %w", err)
	}

	m.logger.Info("MySQL: Deleted database successfully", "db", dbName, "user", user)
	return nil
}
