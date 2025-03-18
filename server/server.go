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
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// Server struct to encapsulate server state
type Server struct {
	PID         int
	IsLeader    bool
	DB          *sql.DB
	LogDir      string
	LogIndex    int
	LogMutex    sync.Mutex
	BackupNodes []string
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

// Response from backup nodes
type ReplicationResponse struct {
	Success   bool   `json:"success"`
	LastIndex int    `json:"last_index"`
	Message   string `json:"message,omitempty"`
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

// Initialize creates a new server instance
func Initialize(PID int) *Server {
	// Init Server
	server := &Server{
		PID:         PID,
		IsLeader:    (PID == 0), // Node with PID 0 is the leader
		BackupNodes: make([]string, 0),
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

// Method to replicate to backup nodes
func (s *Server) ReplicateToBackups(entry LogEntry) {
	if !s.IsLeader || len(s.BackupNodes) == 0 {
		return
	}

	req := ReplicationRequest{
		Entries: []LogEntry{entry},
	}

	for _, addr := range s.BackupNodes {

		client, err := rpc.Dial("tcp", addr)
		if err != nil {
			log.Printf("Node %d: Failed to connect to backup %s: %v", s.PID, addr, err)
			continue
		}

		var resp ReplicationResponse
		err = client.Call("ReplicationHandler.ApplyEntries", req, &resp)
		client.Close()

		if err != nil {
			log.Printf("Node %d: Failed to replicate to %s: %v", s.PID, addr, err)
		} else if !resp.Success {
			log.Printf("Node %d: Replication to %s failed: %s", s.PID, addr, resp.Message)
		}
	}
}

// Method to set backup nodes
func (s *Server) SetBackupNodes(addresses []string) {
	if s.IsLeader {
		s.BackupNodes = addresses
		// log.Printf("Leader node %d will replicate to: %v", s.PID, s.BackupNodes)
	}
}

// Handler for RPC connections
func (s *Server) HandleRPC(rpc_address string) {
	// Create a new RPC server for this instance
	rpcServer := rpc.NewServer()

	// Register handlers with this specific server
	rpcServer.RegisterName("MessageHandler", &MessageHandler{server: s})
	rpcServer.RegisterName("ReplicationHandler", &ReplicationHandler{server: s})

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
func spawn_server(PID int, port int) *Server {
	// log.Printf("Spawning server %d on port %d", PID, port)

	// Initialize the server
	server := Initialize(PID)
	if server == nil {
		log.Fatal("FATAL ERROR ON INIT")
		os.Exit(-1)
	}

	// Start RPC server
	serveraddress := RPC_ADDRESS + strconv.Itoa(port)
	go server.HandleRPC(serveraddress)

	return server
}

// Debug Function
func (t *MessageHandler) GetNodeInfo(dummy *int, info *NodeInfo) error {
	info.NodeID = t.server.PID
	info.IsLeader = t.server.IsLeader
	return nil
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

	// Process each entry
	for _, entry := range req.Entries {
		// Skip duplicate entries
		if entry.Index <= s.LogIndex {
			log.Printf("Node %d: Skipping duplicate entry %d", s.PID, entry.Index)
			continue
		}

		// Check for gaps in the log
		if entry.Index > s.LogIndex+1 {
			resp.Success = false
			resp.Message = fmt.Sprintf("log gap detected, expected %d, got %d", s.LogIndex+1, entry.Index)
			resp.LastIndex = s.LogIndex
			return nil
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
