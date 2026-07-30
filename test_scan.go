package main

import (
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		panic(err)
	}
	db.Exec("CREATE TABLE t (id INTEGER, val TEXT)")
	db.Exec("INSERT INTO t VALUES (1, NULL)")
	
	var val *string
	err = db.QueryRow("SELECT val FROM t WHERE id=1").Scan(&val)
	fmt.Printf("Scan result: %v\n", err)
	if val == nil {
		fmt.Println("val is nil")
	} else {
		fmt.Printf("val is %q\n", *val)
	}
}
