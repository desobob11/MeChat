package main

import (
  //  "bufio"
    "fmt"
   "encoding/json"
  //  "log"
  "io"
  //  "net"
    "net/http"
    //"strings"
    "sync"
     "github.com/rs/cors"
)


func handle_incoming(w http.ResponseWriter, req *http.Request) {
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

   fmt.Println(data)
    w.WriteHeader(http.StatusOK)
}


func httpThread() {
    serv := http.NewServeMux()
    serv.HandleFunc("/incoming", handle_incoming)
    http.ListenAndServe("127.0.0.1:8090", cors.Default().Handler(serv))
}

func main() {



    var wg sync.WaitGroup
    wg.Add(1) 
    go httpThread()




    wg.Wait()
    
}
