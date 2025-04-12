package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/rs/cors"
	"io"
	_ "log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

var ADDRESS_FILE = "replica_addrs.txt"
var REPLICA_ADDRESSES []ReplicaAddress
var ACTIVE_REPLICA ReplicaAddress

type ReplicaAddress struct {
	Address string
	Port    uint16
}

var rpc_client *rpc.Client

// =================================================
//  RPC INTERFACE
//
//  Both server.go (remote) and
//  client.go (client) possess this interface.
// =================================================

// JSON object, represents chat message receieved by user
type ChatMessage struct {
	Message   string
	Timestamp string
	From      int
	To        int
	Acked     int
}

// JSON object, represents create account request received from user
type CreateAccountMessage struct {
	Email     string
	Password  string
	Firstname string
	Lastname  string
	Descr     string
}

// JSON object, represents details that are relevant to a user, regaring
// other user profiles
type UserProfile struct {
	UserId    int
	Email     string
	Firstname string
	Lastname  string
	Descr     string
}

// JSON object, represents a chat between two users. Used
// for querying sent chat messages
type GetMessagesRequest struct {
	UserId    int
	ContactId int
}

// JSON object, array of user profiles
type Contacts struct {
	ContactList []UserProfile
}

// JSON object, arrat of Chat messages
type MessageList struct {
	Messages []ChatMessage
}

// JSON object, represents email and password provided by user
type LoginMessage struct {
	Email    string
	Password string
}

// JSON object, represents a specific RPC response from a remote
// message handler
type RPCResponse struct {
	Message string
}

// JSON object, represents a request to create a new chat between two users
type AddContactMessage struct {
	UserId    int
	ContactId int
}

// JSON object, user ID number, wrapping in struct is necessary
// for Golang RPC
type IDNumber struct {
	ID int
}

// =================================================



/*
    Function that receives an HTTP request from
*/
func HandleIncoming(w http.ResponseWriter, req *http.Request) {
	var data map[string]interface{} // need me a JSON
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
		Message:   message,
		Timestamp: timestamp,
		From:      from,
		To:        to,
		Acked:     acked,
	}
	var response string
	resp := rpc_client.Call("MessageHandler.SaveMessage", messageToBack, &response)
	if resp != nil {
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
		caller, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", replica.Address, replica.Port), 100*time.Millisecond) // #TODO MAKE FINDING NEW LEADER BETTER

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
		leaderFound = ConfirmLeader() // sets rpc_client
	}
	fmt.Printf("Leader is now: %s:%d\n", ACTIVE_REPLICA.Address, ACTIVE_REPLICA.Port)
	err := rpc_client.Call(funcName, args, reply) // check for highest leader everytime

	return err

}

func RequestToJson(req *http.Request) map[string]interface{} {
	var data map[string]interface{} // need me a JSON
	body_text, read_err := io.ReadAll(req.Body)
	if read_err != nil {
		return nil
	}

	err := json.Unmarshal(body_text, &data)
	if err != nil {
		return nil
	}

	return data
}

func CreateAccount(w http.ResponseWriter, req *http.Request) {
	data := RequestToJson(req)

	email, _ := data["Email"].(string)
	pass, _ := data["Password"].(string)
	first, _ := data["Firstname"].(string)
	last, _ := data["Lastname"].(string)
	descr, _ := data["Descr"].(string)

	h := sha256.New()
	h.Write([]byte(pass))

	hash_pass := hex.EncodeToString(h.Sum(nil))

	messageToBack := &CreateAccountMessage{
		Email:     email,
		Password:  hash_pass,
		Firstname: first,
		Lastname:  last,
		Descr:     descr,
	}

	var response RPCResponse
	resp := RemoteProcedureCall("MessageHandler.CreateAccount", messageToBack, &response)

	if resp != nil {
		fmt.Println("Error response from create user RPC ", response)
		w.WriteHeader(http.StatusBadRequest)
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		user_id, _ := strconv.Atoi(response.Message)
		profileResponse := UserProfile{
			UserId:    user_id,
			Email:     email,
			Firstname: first,
			Lastname:  last,
			Descr:     descr,
		}
		json.NewEncoder(w).Encode(profileResponse)
	}
}

func AddContact(w http.ResponseWriter, req *http.Request) {
	data := RequestToJson(req)

	user, _ := data["UserId"].(float64)
	contact, _ := data["ContactId"].(float64)
	int_user := int(user)
	int_contact := int(contact)

	messageToBack := &AddContactMessage{
		UserId:    int_user,
		ContactId: int_contact,
	}

	var response AddContactMessage
	resp := RemoteProcedureCall("MessageHandler.AddContact", messageToBack, &response)

	if resp != nil {
		fmt.Println("Error adding contact: ", response)
		w.WriteHeader(http.StatusBadRequest)
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
	}

}

func Login(w http.ResponseWriter, req *http.Request) {
	data := RequestToJson(req)

	email, _ := data["Email"].(string)
	pass, _ := data["Password"].(string)

	h := sha256.New()
	h.Write([]byte(pass))

	hash_pass := hex.EncodeToString(h.Sum(nil))
	messageToBack := &LoginMessage{
		Email:    email,
		Password: hash_pass,
	}

	var response UserProfile
	err := RemoteProcedureCall("MessageHandler.Login", messageToBack, &response)
	fmt.Println(err)
	fmt.Println(response.UserId)

	if err != nil {
		fmt.Println("Error response from create user RPC ", response)
		w.WriteHeader(http.StatusBadRequest)
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		profileResponse := UserProfile{
			UserId:    response.UserId,
			Email:     response.Email,
			Firstname: response.Firstname,
			Lastname:  response.Lastname,
			Descr:     response.Descr,
		}
		json.NewEncoder(w).Encode(profileResponse)
	}
}

func GetContacts(w http.ResponseWriter, req *http.Request) {
	data := RequestToJson(req)

	userid := int(data["UserId"].(float64))

	messageToBack := &UserProfile{
		UserId: userid,
	}

	var response Contacts
	resp := RemoteProcedureCall("MessageHandler.GetContacts", messageToBack, &response)

	if resp != nil {
		fmt.Println(response)
		w.WriteHeader(http.StatusBadRequest)
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response.ContactList)
	}
}

func GetAllUsers(w http.ResponseWriter, req *http.Request) {
	data := RequestToJson(req)

	userid := int(data["UserId"].(float64))

	messageToBack := &UserProfile{
		UserId: userid,
	}

	var response Contacts
	resp := RemoteProcedureCall("MessageHandler.GetAllUsers", messageToBack, &response)

	if resp != nil {
		fmt.Println(response)
		w.WriteHeader(http.StatusBadRequest)
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response.ContactList)
	}
}

func GetMessages(w http.ResponseWriter, req *http.Request) {
	data := RequestToJson(req)

	userid := int(data["UserId"].(float64))
	contactid := int(data["ContactId"].(float64))

	messageToBack := &GetMessagesRequest{
		UserId:    userid,
		ContactId: contactid,
	}

	var response MessageList
	resp := RemoteProcedureCall("MessageHandler.GetMessages", messageToBack, &response)

	if resp != nil {
		fmt.Println(response)
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
	serv.HandleFunc("/addcontact", AddContact)
	http.ListenAndServe("127.0.0.1:8090", cors.Default().Handler(serv))
}

// function to read in hard-saved replica addresses
func ReadReplicaAddresses(filename string) []ReplicaAddress {
	var addrs []ReplicaAddress
	bytes, _ := os.ReadFile(filename)

	text := string(bytes)
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")

	for _, replica := range lines {
		addr := strings.Split(replica, ":")[0]
		port, _ := strconv.ParseUint(strings.Split(replica, ":")[1], 10, 16)
		addrs = append(addrs, ReplicaAddress{addr, uint16(port)})
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
