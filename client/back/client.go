package main

import (
	//  "bufio"
	"encoding/json"
	"fmt"
	"log"

	//  "log"
	"io"
	//  "net"
	"net/http"
	//"strings"
	"database/sql"
	"net/rpc"
	"sync"

	"github.com/rs/cors"
)



type ChatMessage struct {
    Message string
    Timestamp string
    From int         // email = key?
    To int           // email = key?
	Acked int			// bool 0 | 1
}

type MessageHandler struct {
	mutex sync.Mutex
    db *sql.DB
}


const RPC_ADDRESS = "127.0.0.1:9999"    // localhost for now


func HandleIncoming(w http.ResponseWriter, req *http.Request) {
   var data map[string]interface{}      // need me a JSON

   body_text, read_err := io.ReadAll(req.Body)
    if read_err != nil {
        http.Error(w, "Failure reading request", http.StatusBadRequest)
        return
    }

   err := json.Unmarshal(body_text, &data)
   if err != nil {
       http.Error(w, "Failure converting request to JSON", http.StatusBadRequest)
       return
   }

   message, _ := data["msg"].(string)
   timestamp, _ := data["timestamp"].(string)
   from, _ := data["from"].(int)
   to, _ := data["to"].(int)

   messageToBack := &ChatMessage{
    Message:       message,
    Timestamp: timestamp,
    From:      from,
    To:        to,
    Acked:    0,
}

   var response string

   rpc_client.Go("MessageHandler.SaveMessage", messageToBack, &response, nil)
   if response != ""  {
    log.Fatal("Error response from SaveMessage RPC ", response)
   }

   fmt.Println(data)
    w.WriteHeader(http.StatusOK)
}


func HTTPThread() {
    serv := http.NewServeMux()
    serv.HandleFunc("/incoming", HandleIncoming)
    http.ListenAndServe("127.0.0.1:8090", cors.Default().Handler(serv))
}


var rpc_client *rpc.Client


func main() {
    var err error
    rpc_client, err = rpc.Dial("tcp", RPC_ADDRESS)
    if err != nil {
        log.Fatal("Failed to connect to RPC", err)
    }

    fmt.Println("RPC connection succeeded.")
    fmt.Println(rpc_client)
    var wg sync.WaitGroup
    wg.Add(1) 
    go HTTPThread()




    wg.Wait()
    
}
