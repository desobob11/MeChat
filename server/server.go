package server

/*
	Main drive for server source code.

	Initialization of replica server state is handled here,
	structs are defined, and helper functions are provided.

*/

import (
	"database/sql"
	"fmt"
	"log"
	_ "modernc.org/sqlite"
	"net"
	"net/rpc"
	"os"
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

// LogStatus for consistent logs
type LogStatus struct {
	LogIndex int
}

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

// Used during heartbeat ping, leader should provide
// its ID as an ACK
type IDNumber struct {
	ID int
}

type TimeStamp struct {
	ID    int
	UTC   time.Time     // Replica -> Leader time
	Delta time.Duration // Leader -> Replica telling them the adjustment to make
}

// Moved from RPCFuncs.go
type MessageHandler struct {
	mutex  sync.Mutex
	server *Server
}

// Replication handler for backup nodes
type ReplicationHandler struct {
	mutex  sync.Mutex
	server *Server
}

// Server's PID, based on line of address in address file
var ADDRESS_OFFSET uint32

// changes during synchronization process
var TIMESTAMP_OFFSET int = 0

// store of replica addresses at startup
var ADDRESS_FILE = "replica_addrs.txt"

var REPLICA_ADDRESSES []ReplicaAddress

// MessageHandler, same as defined in RPCFuncs.go.
// Defined as a global for easy access
var messageHandler MessageHandler


// ================================================================
//			GENERAL UTILITY FUNCTIONS
// ================================================================

/*
	Function that compares equality of ReplicaAddresses.
*/
func IsAddressSelf(addr1, addr2 ReplicaAddress) bool {
	return fmt.Sprintf("%s:%d", addr1.Address, addr1.Port) == fmt.Sprintf("%s:%d", addr2.Address, addr2.Port)
}


/*
	Upon receipt of an RPC call, this function
	will cache the caller's IP and port

	Field is primary key, so duplication handling
	is handled by Sqlite
*/
func (s *ReplicationHandler) cacheIP(conn net.Conn) error {
	// need to acquire DB write lock to not conflict with SQLite writes
	// raw SQL script to insert message
	script := `INSERT INTO ip (
			[addr]) 
			VALUES (?);`

	// execute script against database
	messageHandler.mutex.Lock()
	_, err := s.server.DB.Exec(script, conn.RemoteAddr().String())
	messageHandler.mutex.Unlock()
	// handle error
	if err != nil {
		//fmt.Println("Error caching ip: likely duplicate ") // should print out rows changed here eventually
	}
	return err
}

/*
	This function is called by the leader
	upon the completion of an election.

	All cached historical client addresses are sent
	the leader's current address so they can communicate
	directly with the leader.

	Broadcasts are executed in different threads with a lambda function
	defined in this function. Offlince clients are ignored
*/
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
		if err != nil {
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
		}(ip)
	}
	fmt.Println("Address update sent to clients")
	return nil
}






/*
	Called once by a replica upon booting up.

	Polls list of possible replica addresses to find the leader
	(active replica with highest ID)
*/	
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



// =================================================
// 	REPLICA INITIALIZATION FUNCTIONS
// =================================================

/*
This function performs multiple initialization steps including:
 1. Building the server state struct
 2. Creating a local log directory
 3. Kicking off the database schema initilization function in RPCFuncs.go
    if the SQLite Database does not exist yet
*/
func Initialize(PID int) *Server {
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
		// cross platform check here, based on platform
		// address file may contain unexpected newline character at EOF
		if len(strings.Split(replica, `:`)) == 1 {
			continue
		}
		addr := strings.Split(replica, ":")[0]
		port, _ := strconv.ParseUint(strings.Split(replica, ":")[1], 10, 16)
		addrs = append(addrs, ReplicaAddress{addr, uint16(port)})
	}

	return addrs
}

/*
Registers RPC objects and starts/runs the RPC server

This function is intended to be run on a separate thread
*/
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
			fmt.Println("Failure accepting RPC call:", err)
			continue
		}
		go rpcServer.ServeConn(conn)
		go rep.cacheIP(conn)

	}
}


/*
	Function to kick-off server state initialization
*/
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

	server := spawn_server(int(ADDRESS_OFFSET))

	messageHandler = MessageHandler{server: server}
	replicationHandler := ReplicationHandler{server: server}

	// Start RPC server
	go server.HandleRPC(fmt.Sprintf(":%d", server.AddressPort.Port), &messageHandler, &replicationHandler)

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
	} else {
		// let any clients know that we are the leader
		server.SendLeaderAddressToClients()
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
