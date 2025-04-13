package main

import (
	"crypto/sha256"
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
//  HELPER FUNCTIONS
// =================================================

/*
Function that acts as a wrapper around a Golang
RPC call.
*/
func RemoteProcedureCall(funcName string, args any, reply any) error {

	err := rpc_client.Call(funcName, args, reply) // check for highest leader everytime

	return err

}

/*
Function that converts a JSON object from an HTTP
request into a Golang map
*/
func RequestToJson(req *http.Request) map[string]interface{} {
	// map to store results
	var data map[string]interface{}

	// read object from HTTP
	body_text, read_err := io.ReadAll(req.Body)
	if read_err != nil {
		return nil
	}

	// unmartial into map using json lib
	err := json.Unmarshal(body_text, &data)
	if err != nil {
		return nil
	}

	return data
}

/*
Function to read available backend IP/ports
for possible backend replicas.

These addresses are stored locally on machine,
and may be updated by notifications from RPC
*/
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

// =================================================
//  HTTP ENDPOINT FUNCTIONS
// =================================================

/*
Function that receives a chat message from the front-end
UI user. Received over HTTP.

RPC is made to remote Golang server to submit message to backend
*/
func HandleIncoming(w http.ResponseWriter, req *http.Request) {

	// we expect a JSON object
	var data map[string]interface{} // we expect

	// try reading the JSON object
	body_text, read_err := io.ReadAll(req.Body)
	if read_err != nil {
		http.Error(w, "Failure reading request", http.StatusBadRequest)
		return
	}

	// convert object into Golang map
	err := json.Unmarshal(body_text, &data)
	if err != nil {
		http.Error(w, "Failure converting request to JSON", http.StatusBadRequest)
		return
	}

	// parse out fields from map
	message, _ := data["Message"].(string)
	timestamp, _ := data["Timestamp"].(string)
	from := int(data["From"].(float64))
	to := int(data["To"].(float64))
	acked := 1

	// instantiate out ChatMessage for RPC call
	messageToBack := &ChatMessage{
		Message:   message,
		Timestamp: timestamp,
		From:      from,
		To:        to,
		Acked:     acked,
	}

	// string response expected from remote
	var response string

	// make RPC call with message, store result in response
	resp := rpc_client.Call("MessageHandler.SaveMessage", messageToBack, &response)

	// resp is either error or nil
	if resp != nil {
		fmt.Println("Error response from SaveMessage RPC ", resp)
		w.WriteHeader(http.StatusBadRequest)
	} else {
		fmt.Println(data)
		w.WriteHeader(http.StatusOK)
	}

}

// TODO: Skip for now Should this be done differently? Do we want backend to notify
//
//	client of leader change?
func ConfirmLeader() bool {
	// check possible leaders one at a time, starting from highest ID
	// first contact we make is leader by definition (highest active server)
	for i := len(REPLICA_ADDRESSES) - 1; i >= 0; i-- {
		replica := REPLICA_ADDRESSES[i]
		caller, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", replica.Address, replica.Port), 1*time.Second)

		if err == nil {
			var pid IDNumber
			msg := IDNumber{-1}
			client := rpc.NewClient(caller)
			client.Call("MessageHandler.GetPID", msg, &pid)
			ACTIVE_REPLICA = REPLICA_ADDRESSES[pid.ID]
			rpc_client, _ = rpc.Dial("tcp", fmt.Sprintf("%s:%d", ACTIVE_REPLICA.Address, ACTIVE_REPLICA.Port))
			fmt.Printf("Leader is now: %s:%d\n", ACTIVE_REPLICA.Address, ACTIVE_REPLICA.Port)
			return true	// leader found
		}
	}
	fmt.Printf("No leader found yet")
	return false		// no leader found


}

/*
HTTP endpoint function. Receives account create request from user,
relays request to remote over RPC, and returns result
of database operation
*/
func CreateAccount(w http.ResponseWriter, req *http.Request) {
	// convert JSON object, into map, parse out values
	data := RequestToJson(req)
	email, _ := data["Email"].(string)
	pass, _ := data["Password"].(string)
	first, _ := data["Firstname"].(string)
	last, _ := data["Lastname"].(string)
	descr, _ := data["Descr"].(string)

	// store passwords as sha256 digests
	h := sha256.New()
	h.Write([]byte(pass))
	hash_pass := hex.EncodeToString(h.Sum(nil))

	// instantiate message struct for use in RPC
	messageToBack := &CreateAccountMessage{
		Email:     email,
		Password:  hash_pass,
		Firstname: first,
		Lastname:  last,
		Descr:     descr,
	}

	// make RPC call, store response
	var response RPCResponse
	resp := RemoteProcedureCall("MessageHandler.CreateAccount", messageToBack, &response)

	// if there was an error, return error HTTP request
	if resp != nil {
		fmt.Println("Error response from create user RPC ", response)
		w.WriteHeader(http.StatusBadRequest)
	} else { // else send HTTP 200 OK, send back user info including new user ID
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

/*
HTTP endpoint function. Receives an 'add contact'request from user,
relays request to remote over RPC, and returns result
of database operation
*/
func AddContact(w http.ResponseWriter, req *http.Request) {
	// parse HTTP request, convert JSON string to map
	data := RequestToJson(req)
	user, _ := data["UserId"].(float64)
	contact, _ := data["ContactId"].(float64)
	int_user := int(user)
	int_contact := int(contact)

	// instantiate RPC message
	messageToBack := &AddContactMessage{
		UserId:    int_user,
		ContactId: int_contact,
	}

	// make RPC call with remote backend
	var response AddContactMessage
	resp := RemoteProcedureCall("MessageHandler.AddContact", messageToBack, &response)

	// handle errors
	if resp != nil {
		fmt.Println("Error adding contact: ", response)
		w.WriteHeader(http.StatusBadRequest)
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
	}
}

/*
HTTP endpoint function. Receives a 'login' request from user,
relays request to remote over RPC, and returns result
of database operation
*/
func Login(w http.ResponseWriter, req *http.Request) {
	// unmarshall JSON object from request
	data := RequestToJson(req)
	email, _ := data["Email"].(string)
	pass, _ := data["Password"].(string)

	// calculate sha256 digest of attempted password
	h := sha256.New()
	h.Write([]byte(pass))
	hash_pass := hex.EncodeToString(h.Sum(nil))

	// instantiate message for RPC request
	messageToBack := &LoginMessage{
		Email:    email,
		Password: hash_pass,
	}

	// make RPC call
	var response UserProfile
	err := RemoteProcedureCall("MessageHandler.Login", messageToBack, &response)

	// handle errors, send appropriate HTTP respone to user webapp UI
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

/*
HTTP endpoint function. Receives an 'add contact'request from user,
relays request to remote over RPC, and returns result
of database operation

Returns list of user's contacts to user client via HTTP
*/
func GetContacts(w http.ResponseWriter, req *http.Request) {
	// process request message from user
	data := RequestToJson(req)
	userid := int(data["UserId"].(float64))
	messageToBack := &UserProfile{
		UserId: userid,
	}

	// ask RPC for contacts of this user id
	var response Contacts
	resp := RemoteProcedureCall("MessageHandler.GetContacts", messageToBack, &response)

	// handle errors, relay contact list from RPC if HTTP 200 OK
	if resp != nil {
		fmt.Println(response)
		w.WriteHeader(http.StatusBadRequest)
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response.ContactList)
	}
}

/*
HTTP endpoint function. Receives a 'get users' request from UI,
invokes RPC with backend and returns registered users
*/
func GetAllUsers(w http.ResponseWriter, req *http.Request) {
	// parse JSON object into map
	data := RequestToJson(req)

	userid := int(data["UserId"].(float64))

	// instantiate struct for RPC, just need userid
	messageToBack := &UserProfile{
		UserId: userid,
	}

	// invoke RPC
	var response Contacts
	resp := RemoteProcedureCall("MessageHandler.GetAllUsers", messageToBack, &response)

	// handle errors, return user list if HTTP 200 OK
	if resp != nil {
		fmt.Println(response)
		w.WriteHeader(http.StatusBadRequest)
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response.ContactList)
	}
}

/*
HTTP endpoint function. Receives a 'get messages' request from user,
relays request to remote over RPC, and returns result
of database operation

returns all messages between userid and contactid from RPC backend
*/
func GetMessages(w http.ResponseWriter, req *http.Request) {
	// parse JSON request from user
	data := RequestToJson(req)
	userid := int(data["UserId"].(float64))
	contactid := int(data["ContactId"].(float64))

	// message to RPC call
	messageToBack := &GetMessagesRequest{
		UserId:    userid,
		ContactId: contactid,
	}

	// invoke RPC
	var response MessageList
	resp := RemoteProcedureCall("MessageHandler.GetMessages", messageToBack, &response)

	// handle errors
	if resp != nil {
		fmt.Println(response)
		w.WriteHeader(http.StatusBadRequest)
	} else {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response.Messages)
	}
}

/*
Function to handle routing
of HTTP requests.

This function runs an HTTP server
on a separate thread. Endpoints available
to client are registered
with their appropriate RPC call functions.
*/
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



func main() {
	// parse possibl addresses
	REPLICA_ADDRESSES = ReadReplicaAddresses(ADDRESS_FILE)



	// ask back-end for leader address
	leaderFound := ConfirmLeader()
	for !leaderFound {
		leaderFound = ConfirmLeader()
	}

	// output leader ID, debug after connect
	fmt.Printf("Leader: %s:%d\n", ACTIVE_REPLICA.Address, ACTIVE_REPLICA.Port)
	fmt.Println("RPC connection succeeded.")
	fmt.Println(rpc_client)

	// kickoff HTTP thread for client UI
	// communication
	var wg sync.WaitGroup
	wg.Add(1)
	go HTTPThread()
	wg.Wait()

}
