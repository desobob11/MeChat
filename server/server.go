package main

import (
   // "bufio"
    "fmt"
   // "log"
   // "net"
   // "strings"
  //  "net/rpc"
    _ "modernc.org/sqlite"
    "database/sql"
)





func main() {
    // Open (or create) the database file

    var result sql.Result
    db, err := sql.Open("sqlite", "mydatabase.sqlite")
    if err != nil {
        fmt.Println("Error opening database:", err)
        return
    }
    defer db.Close()

    // Create a table
    createTableSQL := `SELECT * FROM users;`
    result, err = db.Exec(createTableSQL)

    fmt.Println(result.RowsAffected())
    
}
