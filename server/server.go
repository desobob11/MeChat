package main

import (
	// "bufio"
	_ "fmt"
	"log"
	"net/rpc"
	"os"

	// "log"
	// "net"
	// "strings"
	//  "net/rpc"
	"database/sql"
	"net"
	"sync"

	_ "modernc.org/sqlite"
)

/*

class Message {
    constructor(msg, timestamp, recv) {
        this.msg = msg;
        this.timestamp = timestamp;
        this.recv = recv;
    }
}
*/


func Initialize() *sql.DB {
    // build database file if we have to
    _, err := os.Stat(DB_NAME)
    if err != nil {
        db, build_err := BuildDatabase()
        if build_err != nil {
            log.Fatal("Error creating database file")
            return nil// failed somewhere
        }
        return db
    }

    db, read_err := sql.Open("sqlite", DB_NAME)
    if read_err != nil {
        log.Fatal("Error opening database file that existed")
        return nil
    }

    return db
}

func HandleRPC() {
    listener, err := net.Listen("tcp", RPC_ADDRESS)
    if err != nil {
        log.Fatal("Failure listening for RPC calls", err)
    }

    for {
		conn, err := listener.Accept()
		if err != nil {
            log.Fatal("Failure accepting RPC call", err)
			continue
		}
		go rpc.ServeConn(conn)
	}
}

var _db *sql.DB

func spawn_server() {
    _db = Initialize()
    if _db == nil {
        log.Fatal("FATAL ERROR ON INIT")
        os.Exit(-1)
    }
    messageHandler := new(MessageHandler)

    rpc.Register(messageHandler)



    var wg sync.WaitGroup
    wg.Add(1) 
    go HandleRPC()

    wg.Wait()
}





/*
    rows := new(sql.Rows)
    db, err := sql.Open("sqlite", "mechat.sqlite")
    if err != nil {
        fmt.Println("Error opening database:", err)
        return
    }
    defer db.Close()

    // Create a table
    query := `SELECT * FROM users;`
    rows, _ = db.Query(query)

    defer rows.Close()
    

    for rows.Next() {
        var id int
        var name string
        var age int

        err = rows.Scan(&id, &name, &age)
        if err != nil {
            fmt.Println("Error scanning row:", err)
            return
        }
        fmt.Printf("ID: %d, Name: %s, Age: %d\n", id, name, age)
    }
*/
    

