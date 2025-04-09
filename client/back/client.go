package main

import (
	//  "bufio"
	"encoding/hex"
	"encoding/json"
	"fmt"
	_ "log"
	"net"
	"strconv"

	//  "log"
	"io"
	//  "net"
	"net/http"
	//"strings"
	"crypto/sha256"
	"database/sql"
	"net/rpc"
	"os"
	"strings"
	"sync"
    "time"
	"github.com/rs/cors"
)

var ADDRESS_FILE = "replica_addrs.txt"
var REPLICA_ADDRESSES []ReplicaAddress
var ACTIVE_REPLICA ReplicaAddress

type ReplicaAddress struct {
	Address string
	Port uint16
}

var rpc_client *rpc.Client

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


type GetMessagesRequest struct {
	UserId int
	ContactId int
}

type Contacts struct {
	ContactList []UserProfile
}

type MessageList struct {
	Messages []ChatMessage
}


type UserProfile struct {
    UserId int
    Email string
    Firstname string
    Lastname string
    Descr string
}

type LoginMessage struct {
	Email string
	Password string
}

type MessageHandler struct {
	mutex sync.Mutex
    db *sql.DB
}

type IDNumber struct {
	ID int
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
   message, _ := data["Message"].(string)
   timestamp, _ := data["Timestamp"].(string)
   from := int(data["From"].(float64))
   to := int(data["To"].(float64))
   acked := 1

   messageToBack := &ChatMessage{
    Message:       message,
    Timestamp: timestamp,
    From:      from,
    To:        to,
    Acked:    acked,
}
    var response string
   resp := rpc_client.Call("MessageHandler.SaveMessage", messageToBack, &response)
   if resp != nil  {
    fmt.Println("Error response from SaveMessage RPC ", resp)
    w.WriteHeader(http.StatusBadRequest)
   } else {
    fmt.Println(data)
    w.WriteHeader(http.StatusOK)
   }

}

func contains(arr []int, target int) bool {
	for _, v := range arr {
		if v == target {
			return true
		}
	}
	return false
}

func ConfirmLeader() bool {
   // leaderIds := []int{}
    leaderId := -1
    for _, replica := range REPLICA_ADDRESSES {
        caller, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", replica.Address, replica.Port), 100 * time.Millisecond)   // #TODO MAKE FINDING NEW LEADER BETTER

        if err == nil {
            var pid IDNumber
            msg := IDNumber{-1}
            client := rpc.NewClient(caller)
            client.Call("MessageHandler.GetPID", msg, &pid)
            if pid.ID > leaderId {
                leaderId = pid.ID
            }
        }
    }
    if leaderId == -1 {
        return false
    }

    ACTIVE_REPLICA = REPLICA_ADDRESSES[leaderId]
    rpc_client, _ = rpc.Dial("tcp", fmt.Sprintf("%s:%d", ACTIVE_REPLICA.Address, ACTIVE_REPLICA.Port))
    return true

}

func RemoteProcedureCall(funcName string, args any, reply any) error {


   leaderFound := ConfirmLeader()
   for !leaderFound {
       leaderFound = ConfirmLeader()       // sets rpc_client
   }
   fmt.Printf("Leader is now: %s:%d\n", ACTIVE_REPLICA.Address, ACTIVE_REPLICA.Port)
   err := rpc_client.Call(funcName, args, reply)      // check for highest leader everytime





    return err
    
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
    resp := RemoteProcedureCall("MessageHandler.CreateAccount", messageToBack, &response)


    if resp != nil  {
     fmt.Println("Error response from create user RPC ", response);
     w.WriteHeader(http.StatusBadRequest)
    } else {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusOK)
        user_id, _ := strconv.Atoi(response.Message)
        profileResponse := UserProfile {
            UserId: user_id,
            Email: email,
            Firstname:  first,
            Lastname:  last,
            Descr: descr,
        }
        json.NewEncoder(w).Encode(profileResponse)
    }
 }




 func Login(w http.ResponseWriter, req *http.Request) {
    data := RequestToJson(req);

    email, _ := data["Email"].(string)
    pass, _ := data["Password"].(string)

    h := sha256.New()
    h.Write([]byte(pass))

    hash_pass := hex.EncodeToString(h.Sum(nil))
    messageToBack := &LoginMessage{
     Email:       email,
     Password: hash_pass,
 }


    var response UserProfile
    err := RemoteProcedureCall("MessageHandler.Login", messageToBack, &response)
    fmt.Println(err)
    fmt.Println(response.UserId)


    if err != nil  {
     fmt.Println("Error response from create user RPC ", response);
     w.WriteHeader(http.StatusBadRequest)
    } else {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusOK)
        profileResponse := UserProfile {
            UserId: response.UserId,
            Email: response.Email,
            Firstname:  response.Firstname,
            Lastname:  response.Lastname,
            Descr: response.Descr,
        }
        json.NewEncoder(w).Encode(profileResponse)
    }
 }


 func GetContacts(w http.ResponseWriter, req *http.Request) {
    data := RequestToJson(req);

    userid := int(data["UserId"].(float64))


    messageToBack := &UserProfile{
     UserId:       userid,
    }

    var response Contacts
    resp := RemoteProcedureCall("MessageHandler.GetContacts", messageToBack, &response)

    if resp != nil  {
     fmt.Println(response);
     w.WriteHeader(http.StatusBadRequest)
    } else {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(response.ContactList)
    }
 }

 func GetAllUsers(w http.ResponseWriter, req *http.Request) {
    data := RequestToJson(req);

    userid := int(data["UserId"].(float64))


    messageToBack := &UserProfile{
     UserId:       userid,
    }

    var response Contacts
    resp := RemoteProcedureCall("MessageHandler.GetAllUsers", messageToBack, &response)

    if resp != nil  {
     fmt.Println(response);
     w.WriteHeader(http.StatusBadRequest)
    } else {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(response.ContactList)
    }
 }


 func GetMessages(w http.ResponseWriter, req *http.Request) {
    data := RequestToJson(req);

    userid := int(data["UserId"].(float64))
    contactid := int(data["ContactId"].(float64))

    messageToBack := &GetMessagesRequest{
     UserId:       userid,
     ContactId: contactid,
    }

    var response MessageList
    resp := RemoteProcedureCall("MessageHandler.GetMessages", messageToBack, &response)

    if resp != nil  {
     fmt.Println(response);
     w.WriteHeader(http.StatusBadRequest)
    } else {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusOK)
        json.NewEncoder(w).Encode(response.Messages)
    }
 }



 

func HTTPThread() {
    serv := http.NewServeMux()
    serv.HandleFunc("/incoming", HandleIncoming)
    serv.HandleFunc("/register", CreateAccount)
    serv.HandleFunc("/login", Login)
    serv.HandleFunc("/getcontacts", GetContacts)
    serv.HandleFunc("/getmessages", GetMessages)
    serv.HandleFunc("/allusers", GetAllUsers)
    http.ListenAndServe("127.0.0.1:8090", cors.Default().Handler(serv))
}



// function to read in hard-saved replica addresses
func ReadReplicaAddresses(filename string) []ReplicaAddress {
	var addrs []ReplicaAddress
	bytes, _ := os.ReadFile(filename);

	text := string(bytes)
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")

	for _, replica := range lines {
		addr := strings.Split(replica, ":")[0]
		port, _ := strconv.ParseUint(strings.Split(replica, ":")[1], 10, 16)
		addrs = append(addrs, ReplicaAddress {addr, uint16(port)})
	}

	return addrs
}



func main() {

    REPLICA_ADDRESSES = ReadReplicaAddresses(ADDRESS_FILE)
    //ACTIVE_REPLICA = REPLICA_ADDRESSES[0]

   // var err error
    //rpc_client, err = rpc.Dial("tcp", fmt.Sprintf("%s:%d", ACTIVE_REPLICA.Address, ACTIVE_REPLICA.Port))
    //if err != nil {
    //    log.Fatal("Failed to connect to RPC", err)
    //}

    leaderFound := ConfirmLeader()
    for !leaderFound {
        leaderFound = ConfirmLeader()
    }
    fmt.Printf("Leader: %s:%d\n", ACTIVE_REPLICA.Address, ACTIVE_REPLICA.Port)
    fmt.Println("RPC connection succeeded.")
    fmt.Println(rpc_client)
    var wg sync.WaitGroup
    wg.Add(1) 
    go HTTPThread()




    wg.Wait()
    
}
