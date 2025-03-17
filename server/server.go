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
	"strconv"
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

var _PID int

func Initialize(PID int) *sql.DB {
    // build database file if we have to
    _PID = PID
    server_database := GenerateDatabaseName(PID)
    _, err := os.Stat(server_database)
    if err != nil {
        db, build_err := BuildDatabase(server_database)
        if build_err != nil {
            log.Fatal("Error creating database file")
            return nil// failed somewhere
        }
        return db
    }

    db, read_err := sql.Open("sqlite", server_database)
    if read_err != nil {
        log.Fatal("Error opening database file that existed")
        return nil
    }

    return db
}

func HandleRPC(rpc_address string) {
    listener, err := net.Listen("tcp", rpc_address)
    println("Listening on", rpc_address)
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

func spawn_server(PID int, port int) {
    println("Spawning server", PID)
    _db = Initialize(PID)
    _serveraddress := RPC_ADDRESS + strconv.Itoa(port)
    if _db == nil {
        log.Fatal("FATAL ERROR ON INIT")
        os.Exit(-1)
    }
    messageHandler := new(MessageHandler)

    rpc.Register(messageHandler)



    var wg sync.WaitGroup
    wg.Add(1) 
    println("Starting RPC server, listening on port ", _serveraddress)
    go HandleRPC(_serveraddress)

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
    

