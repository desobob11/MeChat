package main

import (
	//  "bufio"
	"encoding/json"
	"fmt"
	"log"

    "encoding/hex"
	//  "log"
	"io"
	//  "net"
	"net/http"
	//"strings"
	"database/sql"
	"net/rpc"
	"sync"
    "crypto/sha256"

	"github.com/rs/cors"
)



type ChatMessage struct {
    Message string
    Timestamp string
    From int         // email = key?
    To int           // email = key?
	Acked int			// bool 0 | 1
}

type RPCResponse struct {
	Message string
}

type CreateAccountMessage struct {
    Email string
    Password string
    Firstname string         // email = key?
    Lastname string           // email = key?
	Descr string			// bool 0 | 1
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
   resp := rpc_client.Call("MessageHandler.SaveMessage", messageToBack, &response)
   if resp != nil  {
    fmt.Println("Error response from SaveMessage RPC ", response)
    w.WriteHeader(http.StatusBadRequest)
   } else {
    fmt.Println(data)
    w.WriteHeader(http.StatusOK)
   }

}


func RequestToJson(req *http.Request) map[string]interface{} {
    var data map[string]interface{}      // need me a JSON
    body_text, read_err := io.ReadAll(req.Body)
     if read_err != nil {
         return nil
     }
 
    err := json.Unmarshal(body_text, &data)
    if err != nil {
        return nil
    }
    
    return data;
}

func CreateAccount(w http.ResponseWriter, req *http.Request) {
    data := RequestToJson(req);



    email, _ := data["Email"].(string)
    pass, _ := data["Password"].(string)
    first, _ := data["Firstname"].(string) 
    last, _ := data["Lastname"].(string)
    descr, _ := data["Descr"].(string)

    h := sha256.New()
    h.Write([]byte(pass))

    hash_pass := hex.EncodeToString(h.Sum(nil))

    messageToBack := &CreateAccountMessage{
     Email:       email,
     Password: hash_pass,
     Firstname:      first,
     Lastname:        last,
     Descr:    descr,
 }


    var response RPCResponse
    resp := rpc_client.Call("MessageHandler.CreateAccount", messageToBack, &response)

    
    if resp != nil  {
     fmt.Println("Error response from create user RPC ", response);
     w.WriteHeader(http.StatusBadRequest)
    } else {
        w.WriteHeader(http.StatusOK)
        w.Write([]byte(response.Message))           // give userid back to front end
    }
 }


func HTTPThread() {
    serv := http.NewServeMux()
    serv.HandleFunc("/incoming", HandleIncoming)
    serv.HandleFunc("/register", CreateAccount)
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
