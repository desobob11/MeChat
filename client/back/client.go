package main

import (
    "bufio"
    "fmt"
    "log"
    "net"
    "net/http"
    //"strings"
)


func handle_incoming(w http.ResponseWriter, req *http.Request) {
    fmt.Println(req.Body)
}


func handleConnection(conn net.Conn) {
    defer conn.Close()
    for {
        http.Handle("/foo", fooHandler)

        http.HandleFunc("/bar", func(w http.ResponseWriter, r *http.Request) {
            fmt.Fprintf(w, "Hello, %q", html.EscapeString(r.URL.Path))
        })
        
        log.Fatal(http.ListenAndServe(":8080", nil))
        if err != nil {
            log.Println("Connection error:", err)
            return 
        }
        fmt.Print("Message Received: ", message)
    }
}

func main() {
    http.HandleFunc("/hello", hello)
    http.HandleFunc("/headers", headers)

    ln, err := net.Listen("tcp", "localhost:8000")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println("Listening for front-end 8000")

    conn, err := ln.Accept()
    if err != nil {
        log.Println("Connection accept error:", err)
    }

    go handleConnection(conn)
    
}
