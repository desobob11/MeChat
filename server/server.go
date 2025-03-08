package main

import (
    "bufio"
    "fmt"
    "log"
    "net"
    "strings"
)

func handleConnection(conn net.Conn, cons *[]net.Conn) {
    defer conn.Close() // Ensure the connection is closed when the function finishes

    fmt.Println(cons)
    for {
        message, err := bufio.NewReader(conn).ReadString('\n')
        if err != nil {
            log.Println("Connection error:", err)
            return // Exit the loop if there is an error reading from the connection
        }

        fmt.Print("Message Received: ", message)
        newMessage := strings.ToUpper(message)
        for _, i := range *cons {
            if i != conn {
                i.Write([]byte(newMessage + "\n"))
            }
        }
    }
}

func main() {

    cons := new([]net.Conn)

    ln, err := net.Listen("tcp", "localhost:8000")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println("Listening on port 8000")

    // Accept and handle multiple connections in goroutines
    for {
        conn, err := ln.Accept()
        if err != nil {
            log.Println("Connection accept error:", err)
            continue // Continue accepting new connections even if one fails
        }
        *cons = append(*cons, conn)
        // Handle each connection concurrently
        go handleConnection(conn, cons)
    }
}
