package main

/*
	Main source file for back-end server. Contains
	all implementations and logic for distribution

	Contains source code implementing:
	1. Replication
	2. Fault Tolerance + Leader Election
	3. Synchronization + Consistency

*/

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	_ "modernc.org/sqlite"
	"net"
	"net/rpc"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Struct to store address/port for replicas
type ReplicaAddress struct {
	Address string
	Port    uint16
}

type LogStatus struct {
	LogIndex int
}

var ADDRESS_OFFSET uint32
var TIMESTAMP_OFFSET int = 0

// Server struct to encapsulate server state
type Server struct {
	PID             int
	Active          bool // Is server ready to accept connections?
	IsLeader        bool
	DB              *sql.DB
	LogDir          string
	LogIndex        int
	LogMutex        sync.Mutex
	BackupNodes     []ReplicaAddress
	AddressPort     ReplicaAddress
	LeaderID        int
	Running         bool
	TimestampOffset time.Duration
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
	PID     int    `json:"pid"`
	Message string `json:"message"`
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

type TimeStamp struct {
	ID    int
	UTC   time.Time     // Replica -> Leader time
	Delta time.Duration // Leader -> Replica telling them the adjustment to make
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

// Initialize creates a new server instance
func Initialize(PID int) *Server {
	// Init Server

	//newArray := append([]ReplicaAddress{}, REPLICA_ADDRESSES...)
	//otherReplicas := append(newArray[:ADDRESS_OFFSET], newArray[ADDRESS_OFFSET + 1:]...)

	server := &Server{
		PID:             int(ADDRESS_OFFSET),
		IsLeader:        (ADDRESS_OFFSET == uint32((len(REPLICA_ADDRESSES) - 1))), // Node with PID 0 is the leader
		BackupNodes:     REPLICA_ADDRESSES,
		AddressPort:     REPLICA_ADDRESSES[ADDRESS_OFFSET],
		LeaderID:        -1, // leader is always first IP when the service is kicked off
		Running:         false,
		TimestampOffset: time.Duration(TIMESTAMP_OFFSET) * time.Second,
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

func (s *Server) getTime() time.Time {
	// if s.IsLeader {
	// 	return time.Now()
	// }
	// If not the leader, return the UTC time adjusted by the sync offset
	return time.Now().Add(s.TimestampOffset)
}

func (r *ReplicationHandler) GetLogStatus(dummy int, status *LogStatus) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	status.LogIndex = r.server.LogIndex
	return nil
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

	reqs = append(reqs, entry) // add entry to list of messages we need to send

	req := ReplicationRequest{Entries: reqs}
	for _, addr := range s.BackupNodes {

		if IsAddressSelf(s.AddressPort, addr) { // don't replicate to myself
			continue
		}

		addr_string := net.JoinHostPort(addr.Address, fmt.Sprintf("%d", addr.Port))
		caller, err := net.DialTimeout("tcp", addr_string, 3*time.Second) // need a timeout here, else this hangs if backup not reachable

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

func (s *Server) cacheIP(conn net.Conn) error {
	// raw SQL script to insert message
	script := `INSERT INTO ip (
			[addr]) 
			VALUES (?);`

	// execute script against database
	_, err := s.DB.Exec(script, conn.RemoteAddr().String())

	// handle error
	if err != nil {
		//fmt.Println("Error caching ip: likely duplicate ") // should print out rows changed here eventually
	}
	return err
}

func (s *Server) SendLeaderAddressToClients() error {
	fmt.Println("SENDING LEADER ADDRESSES")
	query := `SELECT
	[addr]
	FROM ip;`

	// attempt to query messages
	rows, err := s.DB.Query(query)
	if err != nil {
		fmt.Println("Error getting user IP for leader update", err)
		return err
	}

	// add ChatMessage struct to RPC invoker's reference for
	// each unique message pulled from database
	addrs := []string{}
	for rows.Next() {
		var rec string
		err = rows.Scan(&rec)
		fmt.Printf("SENDING LEADER ADDRESS TO %s\n", rec)
		if err != nil {
			fmt.Println("Error parsing user IPs err")
			rows.Close()
			return err
		}
		addrs = append(addrs, rec)
	}

	// close connection
	rows.Close()

	for _, addr := range addrs {
		ip := strings.Split(addr, `:`)[0]
		//
		go func(ip string) {
			// ignore errors, skip inactive users
			caller, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%s", ip, "59999"), 2*time.Second)
			if err != nil {
				fmt.Println(err)
				return
			}

			leaderAddres := &ReplicaAddress{
				s.AddressPort.Address,
				s.AddressPort.Port,
			}

			// ignore errors, skip inactive users
			var resp ReplicaAddress
			rpc_client := rpc.NewClient(caller)
			rpc_client.Call("LeaderConnManager.ReceiveLeaderAddress", leaderAddres, &resp) // ignore errors, skip inactive users
			fmt.Println("RPC UPDATE COMPLETE")
		}(ip)
	}
	return nil
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
		go s.cacheIP(conn)
		go rpcServer.ServeConn(conn)
		fmt.Println(conn.RemoteAddr().Network())
		fmt.Println(conn.RemoteAddr().String())

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
	filenames, err := os.ReadDir(server.LogDir)
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
	if s.LeaderID == s.PID {
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
	resp.Success = true
	resp.LastIndex = -1
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
		resp.LastIndex = r.server.PID // bullied it
		if !r.server.Running {
			go r.InitiateElection() // THIS SHOULD PROBABLY NOT BE CALLED IN THE RPC...
		}
	}

	
	return nil
}

func (r *ReplicationHandler) BullyAlgorithmThread() {
	for {
		for !r.BullyFailureDetector() { // check for leader every five seconds
			fmt.Printf("Leader %d is online... \n", r.server.LeaderID)
			fmt.Printf("Current time: %s | Offset is %fs \n", r.server.getTime().Format("15:04:05.000"), r.server.TimestampOffset.Seconds())

			time.Sleep(5 * time.Second)
		}

		// leader is dead
		r.InitiateElection()
	}
}

// func (r *ReplicationHandler) InitTime(msg TimeStamp, resp *TimeStamp) error {
// 	// Initialize time for new replica
// 	// Only the leader should be having this function called
// 	resp.Delta = r.server.getTime().Sub(msg.UTC)
// 	return nil
// }

func (r *ReplicationHandler) GetTime(msg TimeStamp, resp *TimeStamp) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	resp.UTC = r.server.getTime()
	return nil
}

func (r *ReplicationHandler) UpdateTime(msg TimeStamp, resp *TimeStamp) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// fmt.Print("Updating time: ", msg.Delta.Seconds(), "\n")
	// Update the server's timestamp offset
	r.server.TimestampOffset += msg.Delta
	return nil
}

func (r *ReplicationHandler) SyncTime() error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Berkley Time Synchronization Algorithm

	rpcClients := make([]*rpc.Client, 0, len(r.server.BackupNodes))

	for _, addr := range r.server.BackupNodes {
		if IsAddressSelf(r.server.AddressPort, addr) { // skip self
			continue
		}
		caller, err := net.DialTimeout("tcp", net.JoinHostPort(addr.Address, fmt.Sprintf("%d", addr.Port)), 1*time.Second)
		if err != nil {
			continue
		}
		rpcClients = append(rpcClients, rpc.NewClient(caller))
	}

	if len(rpcClients) == 0 {
		fmt.Printf("Error: No backup nodes available for time sync\n")
		return nil
	}
	fmt.Printf("Syncing time with %d nodes\n", len(rpcClients))

	predictedClientTimes := make([]time.Time, len(rpcClients))
	serverTimes := make([]time.Time, len(rpcClients))
	avgs := make([]time.Duration, len(rpcClients))
	avgSum := time.Duration(0)

	for i, client := range rpcClients {
		var resp TimeStamp
		before := r.server.getTime()
		err := client.Call("ReplicationHandler.GetTime", TimeStamp{}, &resp)
		if err != nil {
			fmt.Printf("Error: %s", err)
			continue
		}
		after := r.server.getTime()
		flightTime := after.Sub(before) / 2
		predictedTime := resp.UTC.Add(flightTime)
		predictedClientTimes[i] = predictedTime
		serverTimes[i] = after

		avgs[i] = predictedTime.Sub(after)
		avgSum += predictedTime.Sub(after)
	}

	avgOffset := avgSum / time.Duration(len(rpcClients)+1) // Add 1 to the number of clients to include the leader's time in the average. Leader has 0 offset from itself though

	r.server.TimestampOffset += avgOffset

	for i, client := range rpcClients {
		delta := avgOffset - avgs[i]
		fmt.Printf("Telling node %d to update time by %f, avgOffset=%f, it's offset = %f \n", i, delta.Seconds(), avgOffset.Seconds(), avgs[i].Seconds())
		client.Call("ReplicationHandler.UpdateTime", TimeStamp{Delta: delta}, TimeStamp{})
	}

	return nil
}

func (r *MessageHandler) GetPID(msg *IDNumber, resp *IDNumber) error {
	resp.ID = r.server.PID
	return nil
}

func SendBullyMessage(replica ReplicaAddress, funcName string, msg BullyMessage, resp *ReplicationResponse) error {
	addr_string := net.JoinHostPort(replica.Address, fmt.Sprintf("%d", replica.Port))
	caller, err := net.DialTimeout("tcp", addr_string, 1*time.Second) // need a timeout here, else this hangs if backup not reachable
	if err != nil {
		fmt.Printf("Replica at %s is offline\n", addr_string)
		return err
	}
	client := rpc.NewClient(caller)
	err = client.Call(fmt.Sprintf("ReplicationHandler.%s", funcName), msg, resp) // skipping error here as well
	return err
}

func (r *ReplicationHandler) InitiateElection() bool {
	r.server.Running = true
	fmt.Println("CALLING ELECTION")

	if r.server.PID == len(r.server.BackupNodes)-1 {
		for _, replica := range r.server.BackupNodes {
			if IsAddressSelf(r.server.AddressPort, replica) { // skip crashed leader and self
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
			if j <= r.server.PID { // skip crashed leader and self
				continue
			}
			msg := BullyMessage{PID: r.server.PID, Message: "ELECTION"}
			SendBullyMessage(replica, "BullyElection", msg, &electionResponse)

		}
		time.Sleep(1 * time.Second) // probably much too long

		if electionResponse.LastIndex == -1 { // no response
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
	if (r.server.LeaderID == r.server.PID) {
		r.server.SendLeaderAddressToClients()
	}
	return false
}

func (r *ReplicationHandler) SyncLogs() error {
	localIndex := r.server.LogIndex

	rpcClients := make([]*rpc.Client, 0, len(r.server.BackupNodes))

	for _, addr := range r.server.BackupNodes {
		if IsAddressSelf(r.server.AddressPort, addr) { // skip self
			continue
		}

		rpcAddr := net.JoinHostPort(addr.Address, fmt.Sprintf("%d", addr.Port))
		caller, err := net.DialTimeout("tcp", rpcAddr, 1*time.Second)
		if err != nil {
			// log.Printf("Node %d: Cannot reach %s: %s", r.server.PID, rpcAddr, err)
			continue
		}
		rpcClients = append(rpcClients, rpc.NewClient(caller))
	}

	if len(rpcClients) == 0 {
		fmt.Printf("Error: No backup nodes available for log  sync\n")
		return nil
	}

	for i, client := range rpcClients {
		var status LogStatus
		err := client.Call("ReplicationHandler.GetLogStatus", 0, &status)

		if err != nil {
			fmt.Printf("Error: %s", err)
			continue
		}

		// client.Close()?

		if status.LogIndex < localIndex {
			fmt.Printf("Telling node %d to update its logs\n", i)

			var resp IDNumber

			if err := r.CatchupReplica(IDNumber{ID: i}, &resp); err != nil {
				fmt.Printf("Replica %d: Error during CatchupReplica: %v\n", i, err)
				continue
			}

			if resp.ID == -1 {
				fmt.Printf("Node %d: Logs successfully caught up\n", i)
			} else {
				fmt.Printf("Node %d: Failed to catch up logs\n", i)
			}
		} else if status.LogIndex > localIndex {
			fmt.Printf("Replica %d has a higher log index (%d vs leader %d); replacing logs...\n", i, status.LogIndex, localIndex)

			var eraseResp ReplicationResponse

			// erase logs
			if err := client.Call("ReplicationHandler.EraseLogsFromDir", 0, &eraseResp); err != nil {
				fmt.Printf("Replica %d: Error calling EraseLogsFromDir: %v\n", i, err)
				continue
			}

			// make sure erase was successful
			if !eraseResp.Success {
				fmt.Printf("Replica %d: Failed to erase logs: %s\n", i, eraseResp.Message)
				continue
			}

			fmt.Printf("Replica %d: Logs erased successfully.\n", i)

			var catchupResp IDNumber

			if err := r.CatchupReplica(IDNumber{ID: i}, &catchupResp); err != nil {
				fmt.Printf("Replica %d: Error during CatchupReplica: %v\n", i, err)
				continue
			}

			if catchupResp.ID == -1 {
				fmt.Printf("Node %d: Logs successfully replaced.\n", i)
			} else {
				fmt.Printf("Node %d: CatchupReplica did not complete as expected (returned %d).\n", i, catchupResp.ID)
			}
		} else {
			fmt.Printf("Node %d is already up to date\n", i)
		}

	}

	return nil
}

func (r *ReplicationHandler) EraseLogsFromDir(_args int, resp *ReplicationResponse) error {

	// Leader should never delete their logs only replicas
	if r.server.PID == r.server.LeaderID {
		resp.Success = false
		resp.Message = "Leader cannot delete logs"
		return nil
	}

	logDir := r.server.LogDir

	//Delete the logDir which should remove all the replicas logs
	if err := os.RemoveAll(logDir); err != nil {
		resp.Success = false
		resp.Message = fmt.Sprintf("Error deleting log directory: %v", err)
		return err
	}

	// Recreate the log directory, don't need to check if it exists as we just deleted it
	if err := os.MkdirAll(logDir, 0755); err != nil {
		resp.Success = false
		resp.Message = fmt.Sprintf("error creating log directory: %v", err)
		return err
	}

	r.server.LogIndex = 0 // Reset log index after deletion

	resp.Success = true
	return nil
}

func (r *ReplicationHandler) BullyFailureDetector() bool {
	if r.server.PID == r.server.LeaderID {
		// Leader can do a time and log sync here instead of detecting leader failure
		r.SyncTime()
		r.SyncLogs()
		return false // leader can't detect its own failures, return false
	}

	leader_addr := r.server.BackupNodes[r.server.LeaderID]
	addr_string := net.JoinHostPort(leader_addr.Address, fmt.Sprintf("%d", leader_addr.Port))
	t_trans := 50 * time.Millisecond
	t_proc := 10 * time.Millisecond
	t := (2 * t_trans) + t_proc

	caller, err := net.DialTimeout("tcp", addr_string, t) // need a timeout here, else this hangs if backup not reachable
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
	return false
}

func (r *ReplicationHandler) CatchupReplica(msg IDNumber, resp *IDNumber) error {

	reqs, err := ReadAllEntires(r.server)
	if err != nil {
		fmt.Printf("Error: %s", err)
	}

	req := ReplicationRequest{Entries: reqs}
	addr := r.server.BackupNodes[msg.ID]
	//	for _, req := range s.BackupNodes {

	addr_string := net.JoinHostPort(addr.Address, fmt.Sprintf("%d", addr.Port))
	caller, err := net.DialTimeout("tcp", addr_string, 3*time.Second) // need a timeout here, else this hangs if backup not reachable

	if err != nil {
		log.Printf("Node %d: Failed to connect to backup %s: %v", r.server.PID, addr_string, err)
	}

	var to_resp ReplicationResponse
	client := rpc.NewClient(caller)
	err = client.Call("ReplicationHandler.ApplyEntries", req, &to_resp)
	client.Close()

	if err != nil {
		log.Printf("Node %d: Failed to replicate to %s: %v", r.server.PID, addr_string, err)
	} else if !to_resp.Success {
		log.Printf("Node %d: Replication to %s failed: %s", r.server.PID, addr_string, to_resp.Message)
	}
	//}
	resp.ID = -1
	return nil
}

func ConfirmLeader(s *Server) bool {
	// leaderIds := []int{}
	leaderId := -1
	for i, replica := range REPLICA_ADDRESSES {
		if i == s.PID {
			continue
		}
		caller, err := net.DialTimeout("tcp", net.JoinHostPort(replica.Address, fmt.Sprintf("%d", replica.Port)), 1*time.Second)

		if err != nil {
			print("Error: ", err.Error())
		}

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
		s.LeaderID = s.PID
		return false
	}

	s.LeaderID = leaderId
	return true

}

func main() {
	// Configuration

	offset, err := strconv.ParseUint(os.Args[1], 10, 32)

	if err != nil {
		fmt.Println(`Command line error, please run server using this command: go run . <offset> <optional:timestampOffset>\n\n
		Where offset is the 0-indexed number corresponding to the desired address in the address file\n
		And where timestampOffset is the UTC offset in seconds`)
		return
	}

	if len(os.Args) > 2 {
		timestampOffset, err := strconv.ParseInt(os.Args[2], 10, 32)
		if err != nil {
			fmt.Println("Error parsing timestamp offset, please specify integer Timestamp Offset in seconds")
			return
		}
		TIMESTAMP_OFFSET = int(timestampOffset)
	}

	ADDRESS_OFFSET = uint32(offset)

	REPLICA_ADDRESSES = ReadReplicaAddresses(ADDRESS_FILE) // all addresses, including own

	// Start the leader first (PID 0)
	server := spawn_server(0)

	messageHandler := MessageHandler{server: server}
	replicationHandler := ReplicationHandler{server: server}

	// Start RPC server
	go server.HandleRPC(fmt.Sprintf("%s:%d", server.AddressPort.Address, server.AddressPort.Port), &messageHandler, &replicationHandler)

	time.Sleep(1 * time.Second)

	ConfirmLeader(server)
	fmt.Printf("LEADER: %d\n", server.LeaderID)
	if server.LeaderID != server.PID {

		var resp IDNumber
		leader_addr := server.BackupNodes[server.LeaderID]
		addr_string := net.JoinHostPort(leader_addr.Address, fmt.Sprintf("%d", leader_addr.Port))
		caller, r_err := net.DialTimeout("tcp", addr_string, 1*time.Second) // assuming that there will be no error, assuming ConfirmLeader works
		fmt.Println("ERROR ", r_err)

		client := rpc.NewClient(caller)
		client.Call("ReplicationHandler.CatchupReplica", IDNumber{ID: server.PID}, &resp)

		if resp.ID == -1 {
			fmt.Println("Replica caught up")
		}

		// Initialize time for new replica
		// var timeResp TimeStamp
		// client.Call("ReplicationHandler.InitTime", TimeStamp{ID: server.PID, UTC: server.getTime()}, &timeResp)
		// server.TimestampOffset = timeResp.Delta

		client.Close()
	}

	// only kick off election if we are higher PID
	if server.LeaderID < server.PID {
		replicationHandler.InitiateElection()
	}

	go replicationHandler.BullyAlgorithmThread() // NEED TO detect leader failures

	for _, addr := range server.BackupNodes {
		fmt.Printf("%s:%d\n", addr.Address, addr.Port)
	}

	// Give the leader time to initialize
	time.Sleep(1 * time.Second)

	fmt.Printf("Replica %d running at %s:%d\n", ADDRESS_OFFSET, server.AddressPort.Address, server.AddressPort.Port)
	fmt.Printf("SERVER START: Current time: %s | Offset is %fs \n", server.getTime().Format("15:04:05.000"), server.TimestampOffset.Seconds())
	fmt.Println("Clients may now connect")

	// Wait forever
	select {}

}
