package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/rpc"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"sort"

	_ "modernc.org/sqlite"
)
// Struct to store address/port for replicas
type ReplicaAddress struct {
	Address string
	Port uint16
}

var ADDRESS_OFFSET uint32

// Server struct to encapsulate server state
type Server struct {
	PID         int
	IsLeader    bool
	DB          *sql.DB
	LogDir      string
	LogIndex    int
	LogMutex    sync.Mutex
	BackupNodes []ReplicaAddress
	AddressPort ReplicaAddress
	LeaderID int
	Running bool
}

// Type definitions for replication
// Log entry structure
type LogEntry struct {
	Index     int       `json:"index"`
	SQL       string    `json:"sql"`
	Args      []any     `json:"args"`
	Timestamp time.Time `json:"timestamp"`
}

// ReplicationRequest for sending entries to backups
type ReplicationRequest struct {
	Entries []LogEntry `json:"entries"`
}

type BullyMessage struct {
	PID int	`json:pid`
	Message string	`json:message`
}

// Response from backup nodes
type ReplicationResponse struct {
	Success   bool   `json:"success"`
	LastIndex int    `json:"last_index"`
	Message   string `json:"message,omitempty"`
}

type IDNumber struct {
	ID int
}

// Moved from serverUtils
type MessageHandler struct {
	mutex  sync.Mutex
	server *Server
}

// Replication handler for backup nodes
type ReplicationHandler struct {
	mutex  sync.Mutex
	server *Server
}

/*

class Message {
    constructor(msg, timestamp, recv) {
        this.msg = msg;
        this.timestamp = timestamp;
        this.recv = recv;
    }
}
*/

var ADDRESS_FILE = "replica_addrs.txt"
var REPLICA_ADDRESSES []ReplicaAddress

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

// Initialize creates a new server instance
func Initialize(PID int) *Server {
	// Init Server

	//newArray := append([]ReplicaAddress{}, REPLICA_ADDRESSES...)
	//otherReplicas := append(newArray[:ADDRESS_OFFSET], newArray[ADDRESS_OFFSET + 1:]...)
	

	server := &Server{
		PID:         int(ADDRESS_OFFSET),
		IsLeader:    (ADDRESS_OFFSET == uint32((len(REPLICA_ADDRESSES) - 1))), // Node with PID 0 is the leader
		BackupNodes: REPLICA_ADDRESSES,
		AddressPort: REPLICA_ADDRESSES[ADDRESS_OFFSET],
		LeaderID: len(REPLICA_ADDRESSES) - 1,					// leader is always first IP when the service is kicked off
		Running: false,
	}

	// Init Log
	server.LogDir = fmt.Sprintf("logs-node-%d", PID)
	if err := os.MkdirAll(server.LogDir, 0755); err != nil {
		log.Fatal("Error creating log directory:", err)
	}

	// Find highest log index
	files, err := os.ReadDir(server.LogDir)
	if err == nil {
		for _, file := range files {
			if file.IsDir() {
				continue
			}

			var index int
			if _, err := fmt.Sscanf(file.Name(), "log-%d.json", &index); err == nil {
				if index > server.LogIndex {
					server.LogIndex = index
				}
			}
		}
	}

	// log.Printf("Node %d started as %s with log index %d",
	// 	server.PID,
	// 	map[bool]string{true: "LEADER", false: "BACKUP"}[server.IsLeader],
	// 	server.LogIndex)

	// Initialize database
	server_database := GenerateDatabaseName(PID)
	_, err = os.Stat(server_database)
	if err != nil {
		db, build_err := BuildDatabase(server_database)
		if build_err != nil {
			log.Fatal("Error creating database file")
			return nil
		}
		server.DB = db
	} else {
		db, read_err := sql.Open("sqlite", server_database)
		if read_err != nil {
			log.Fatal("Error opening database file that existed")
			return nil
		}
		server.DB = db
	}

	return server
}

// Method to append an entry to the log
func (s *Server) AppendToLog(entry LogEntry) (LogEntry, error) {
	s.LogMutex.Lock()
	defer s.LogMutex.Unlock()

	// Increment log index
	s.LogIndex++
	entry.Index = s.LogIndex
	entry.Timestamp = time.Now()

	// Convert to JSON
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return entry, fmt.Errorf("error serializing log entry: %v", err)
	}

	// Write to file
	filename := filepath.Join(s.LogDir, fmt.Sprintf("log-%d.json", entry.Index))
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return entry, fmt.Errorf("error writing log file: %v", err)
	}

	log.Printf("Node %d: Appended entry %d to log", s.PID, entry.Index)
	return entry, nil
}

func IsAddressSelf(addr1, addr2 ReplicaAddress) bool {
	return fmt.Sprintf("%s:%d", addr1.Address, addr1.Port) == fmt.Sprintf("%s:%d", addr2.Address, addr2.Port)
}



// Method to replicate to backup nodes
func (s *Server) ReplicateToBackups(entry LogEntry) {
	if !s.IsLeader || len(s.BackupNodes) == 0 {
		return
	}

	reqs, err := ReadAllEntires(s)
	if err != nil {
		fmt.Printf("Error: %s", err)
	}

	reqs = append(reqs, entry)	// add entry to list of messages we need to send


	req := ReplicationRequest{Entries: reqs}
	for _, addr := range s.BackupNodes {

		if IsAddressSelf(s.AddressPort, addr) {			// don't replicate to myself
			continue
		}

		addr_string := fmt.Sprintf("%s:%d", addr.Address, addr.Port)
		caller, err := net.DialTimeout("tcp", addr_string, 3*time.Second)		// need a timeout here, else this hangs if backup not reachable

		if err != nil {
			log.Printf("Node %d: Failed to connect to backup %s: %v", s.PID, addr_string, err)
			continue
		}

		var resp ReplicationResponse
		client := rpc.NewClient(caller)
		err = client.Call("ReplicationHandler.ApplyEntries", req, &resp)
		client.Close()

		if err != nil {
			log.Printf("Node %d: Failed to replicate to %s: %v", s.PID, addr_string, err)
		} else if !resp.Success {
			log.Printf("Node %d: Replication to %s failed: %s", s.PID, addr_string, resp.Message)
		}
	}
}

// Method to set backup nodes
func (s *Server) SetBackupNodes(addresses []ReplicaAddress) {
	if s.IsLeader {
		s.BackupNodes = addresses
		// log.Printf("Leader node %d will replicate to: %v", s.PID, s.BackupNodes)
	}
}

// Handler for RPC connections
func (s *Server) HandleRPC(rpc_address string, msg *MessageHandler, rep *ReplicationHandler) {
	// Create a new RPC server for this instance
	rpcServer := rpc.NewServer()

	// Register handlers with this specific server
	rpcServer.RegisterName("MessageHandler", msg)
	rpcServer.RegisterName("ReplicationHandler", rep)

	// Start listening
	listener, err := net.Listen("tcp", rpc_address)
	log.Println("Listening on", rpc_address)
	if err != nil {
		log.Fatal("Failure listening for RPC calls:", err)
	}

	// Handle connections
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Failure accepting RPC call:", err)
			continue
		}
		go rpcServer.ServeConn(conn)
	}
}

// Spawn a server with the given PID and port
func spawn_server(PID int) *Server {
	// log.Printf("Spawning server %d on port %d", PID, port)

	// Initialize the server
	server := Initialize(PID)
	if server == nil {
		log.Fatal("FATAL ERROR ON INIT")
		os.Exit(-1)
	}


	return server
}

// Debug Function
func (t *MessageHandler) GetNodeInfo(dummy *int, info *NodeInfo) error {
	info.NodeID = t.server.PID
	info.IsLeader = t.server.IsLeader
	return nil
}

func ReadAllEntires(server *Server) ([]LogEntry, error) {
	logFiles := []LogEntry{} 
	filenames, err := os.ReadDir(server.LogDir);
	if err != nil {
		fmt.Printf("Error: %s", err)
		return logFiles, err
	}

	for _, name := range filenames {
		text, err := os.ReadFile(fmt.Sprintf("%s/%s", server.LogDir, name.Name()))
		if err != nil {
			fmt.Printf("Error: %s", err)
			return logFiles, err
		}
		var data LogEntry
		err = json.Unmarshal(text, &data)
		if err != nil {
			fmt.Printf("Error: %s", err)
			return logFiles, err
		}
		
		logFiles = append(logFiles, data)

	}
	return logFiles, nil
}

// Handler for replication requests
func (r *ReplicationHandler) ApplyEntries(req *ReplicationRequest, resp *ReplicationResponse) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	s := r.server
	log.Printf("Node %d: Received %d entries for replication", s.PID, len(req.Entries))

	
	// Reject if we're the leader		
	if s.IsLeader {
		resp.Success = false
		resp.Message = "leader cannot accept replication requests"
		return nil
	}

	sort.Slice(req.Entries, func(i, j int) bool {
		return req.Entries[i].Index < req.Entries[j].Index
	})

	// Process each entry
	for _, entry := range req.Entries {
		// Skip duplicate entries
		if entry.Index <= s.LogIndex {
			log.Printf("Node %d: Skipping duplicate entry %d", s.PID, entry.Index)
			continue
		}
		
		// Check for gaps in the log
		if entry.Index > s.LogIndex+1 {
			//resp.Success = false
			//resp.Message = fmt.Sprintf("log gap detected, expected %d, got %d, catching up", s.LogIndex+1, entry.Index)
			fmt.Printf("log gap detected, expected %d, got %d, catching up\n", s.LogIndex+1, entry.Index)
			//resp.LastIndex = s.LogIndex
			//return nil
			s.LogIndex = entry.Index
		}

		// Ensure database connection is valid
		if s.DB == nil {
			resp.Success = false
			resp.Message = "database connection is nil"
			return fmt.Errorf("database connection is nil")
		}

		// Execute the SQL statement
		_, err := s.DB.Exec(entry.SQL, entry.Args...)
		if err != nil {
			resp.Success = false
			resp.Message = fmt.Sprintf("error applying SQL: %v", err)
			return err
		}

		// Save to log
		data, err := json.MarshalIndent(entry, "", "  ")
		if err != nil {
			resp.Success = false
			resp.Message = fmt.Sprintf("error serializing entry: %v", err)
			return err
		}

		// Ensure log directory exists
		if _, err := os.Stat(s.LogDir); os.IsNotExist(err) {
			if err := os.MkdirAll(s.LogDir, 0755); err != nil {
				resp.Success = false
				resp.Message = fmt.Sprintf("error creating log directory: %v", err)
				return err
			}
		}

		// Write to log file
		filename := filepath.Join(s.LogDir, fmt.Sprintf("log-%d.json", entry.Index))
		if err := os.WriteFile(filename, data, 0644); err != nil {
			resp.Success = false
			resp.Message = fmt.Sprintf("error writing log file: %v", err)
			return err
		}

		// Update index
		s.LogIndex = entry.Index
		log.Printf("Node %d: Applied entry %d", s.PID, entry.Index)
	}

	// Success response
	resp.Success = true
	resp.LastIndex = s.LogIndex
	return nil
}


func (r *ReplicationHandler) IsStatusOK(req *ReplicationRequest, resp *ReplicationResponse) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	resp.Success = true;
	resp.LastIndex = -1;
	resp.Message = "STATUSOK"
	return nil
}

/*
	Bully messages below
*/

func (r *ReplicationHandler) BullyLeader(msg *BullyMessage, resp *ReplicationResponse) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.server.LeaderID = msg.PID
	r.server.Running = false
	return nil
}

func (r *ReplicationHandler) BullyElection(msg *BullyMessage, resp *ReplicationResponse) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if msg.PID < r.server.PID {
		resp.LastIndex = r.server.PID		// bullied it
		if !r.server.Running {
			go r.InitiateElection()		// THIS SHOULD PROBABLY NOT BE CALLED IN THE RPC...
		}
	}
	return nil
}


func (r* ReplicationHandler) BullyAlgorithmThread() {
	for {
		for !r.BullyFailureDetector() {	// check for leader every five seconds
			fmt.Printf("Leader %d is online...\n", r.server.LeaderID)
			
			time.Sleep(5 * time.Second)
		}

		// leader is dead
		r.InitiateElection()
	}
}


func (r *MessageHandler) GetPID(msg *IDNumber, resp *IDNumber) error {
	resp.ID = r.server.PID
	return nil
}


func SendBullyMessage(replica ReplicaAddress, funcName string, msg BullyMessage, resp *ReplicationResponse) error {
	addr_string := fmt.Sprintf("%s:%d", replica.Address, replica.Port)
	caller, err := net.DialTimeout("tcp", addr_string, 1*time.Second)		// need a timeout here, else this hangs if backup not reachable
	if err != nil {
		fmt.Printf("Replica at %s is offline\n", addr_string)
		return err
	}
	client := rpc.NewClient(caller)
	err = client.Call(fmt.Sprintf("ReplicationHandler.%s", funcName), msg, resp)		// skipping error here as well
	return err
}


func  (r *ReplicationHandler) InitiateElection() bool {
	r.server.Running = true;
	fmt.Println("CALLING ELECTION")

	if r.server.PID == len(r.server.BackupNodes) - 1 {
		for _, replica := range r.server.BackupNodes {
			if IsAddressSelf(r.server.AddressPort, replica) { 			// skip crashed leader and self
				continue
			}
			var resp ReplicationResponse
			msg := BullyMessage{PID: r.server.PID, Message: "LEADER"}
			r.server.LeaderID = r.server.PID
			r.server.Running = false
			SendBullyMessage(replica, "BullyLeader", msg, &resp)
		}
	} else {
		electionResponse := ReplicationResponse{LastIndex: -1}
		for j, replica := range r.server.BackupNodes {
			if j <= r.server.PID {			// skip crashed leader and self
				continue
			}
			msg := BullyMessage{PID: r.server.PID, Message: "ELECTION"}
			SendBullyMessage(replica, "BullyElection", msg, &electionResponse)
		
		}
		time.Sleep(1 * time.Second)		// probably much too long

		
		if electionResponse.LastIndex == -1 {	// no response
			r.server.LeaderID = r.server.PID
			for j, replica := range r.server.BackupNodes {
				// skip self
				if j == r.server.PID {	
					continue
				}
				msg := BullyMessage{PID: r.server.PID, Message: "LEADER"}
				SendBullyMessage(replica, "BullyLeader", msg, nil)
			}
		} else {
			time.Sleep(1 * time.Second)
			//if r.server.LeaderID == current_leader {			// no leader change
			//	r.InitiateElection()
			//} else {			// other process already told me to update my leader
				r.server.Running = false
			//}
		}

	}

	return false
}


func  (r *ReplicationHandler) BullyFailureDetector() bool {
	if r.server.PID == r.server.LeaderID {
		return false			// leader can't detect its own failures
	}

	leader_addr := r.server.BackupNodes[r.server.LeaderID]
	addr_string := fmt.Sprintf("%s:%d", leader_addr.Address, leader_addr.Port)
	caller, err := net.DialTimeout("tcp", addr_string, 5*time.Second)		// need a timeout here, else this hangs if backup not reachable
	if err != nil {
		fmt.Printf("Leader down: %s\n", err)
		return true
	}

	var resp ReplicationResponse
	var msg ReplicationRequest
	client := rpc.NewClient(caller)
	err = client.Call("ReplicationHandler.IsStatusOK", msg, &resp)
	if err != nil {
		fmt.Printf("Leader down: %s\n", err)
		return true
	}

	if resp.Message != "STATUSOK" {
		fmt.Printf("Leader down: %s\n", err)
		return true
	}
	return false;
}





func main() {
	// Configuration

	
	offset, err := strconv.ParseUint(os.Args[1], 10, 32)

	if err != nil {
		fmt.Println(`Command line error, please run server using this command: go run . <offset>\n\n
					 Where offset is the 0-indexed number corresponding to the desired address in the address file`)
	}

	ADDRESS_OFFSET = uint32(offset)
	
	REPLICA_ADDRESSES = ReadReplicaAddresses(ADDRESS_FILE)	// all addresses, including own



	// Start the leader first (PID 0)
	server := spawn_server(0)

	messageHandler := MessageHandler{server: server}
	replicationHandler := ReplicationHandler{server: server}
	
	// Start RPC server
	go server.HandleRPC(fmt.Sprintf("%s:%d", server.AddressPort.Address, server.AddressPort.Port), &messageHandler, &replicationHandler)

	
	time.Sleep(1 * time.Second)
	replicationHandler.InitiateElection()
	go replicationHandler.BullyAlgorithmThread()		// NEED TO detect leader failures

	for _, addr := range server.BackupNodes {
		fmt.Printf("%s:%d\n", addr.Address, addr.Port)
	}

	// Give the leader time to initialize
	time.Sleep(1 * time.Second)

	fmt.Printf("Replica %d running at %s:%d\n", ADDRESS_OFFSET, server.AddressPort.Address, server.AddressPort.Port)
	fmt.Println("Clients may now connect")

	// Wait forever
	select {}
	

	
}