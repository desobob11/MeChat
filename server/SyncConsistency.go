package server
/*
	Source code that encapsulates synchronization
	and consistency.

	Use of Cristian's algorithm
	for synchronization, and log-based
	messaging for consistency

*/

import (
	"encoding/json"
	"fmt"
	"log"
	_ "modernc.org/sqlite"
	"net"
	"net/rpc"
	"os"
	"path/filepath"
	"sort"
	"time"
)

/*
	Function to get local time of machine
*/
func (s *Server) getTime() time.Time {
	// if s.IsLeader {
	// 	return time.Now()
	// }
	// If not the leader, return the UTC time adjusted by the sync offset
	return time.Now().Add(s.TimestampOffset)
}

/*
	Function to determine if Log has been processed
*/
func (r *ReplicationHandler) GetLogStatus(dummy int, status *LogStatus) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	status.LogIndex = r.server.LogIndex
	return nil
}


/*
	This function takes a processed log (derived from an RPC message from client)
	and records it locally in JSON format
*/
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


/*
	This function propagates a message received from client
	via RPC to other replicas.

	This function is invoked by an RPC function defined in
	RPCFuncs.go
*/
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

/*
	Read all local message logs, return an array of processed
	log messages
*/
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


/*
	RPC method invocation

	A leader will all this on a replica to apply logged entries
	that may have been missed
*/
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
		messageHandler.mutex.Lock()
		_, err := s.DB.Exec(entry.SQL, entry.Args...)
		messageHandler.mutex.Unlock()
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



func (r *ReplicationHandler) CatchupReplica(msg IDNumber, resp *IDNumber) error {

	reqs, err := ReadAllEntires(r.server)
	if err != nil {
		fmt.Printf("Error: %s", err)
	}

	req := &ReplicationRequest{Entries: reqs}
	addr := r.server.BackupNodes[msg.ID]
	//	for _, req := range s.BackupNodes {

	addr_string := net.JoinHostPort(addr.Address, fmt.Sprintf("%d", addr.Port))
	caller, err := net.DialTimeout("tcp", addr_string, 2*time.Second) // need a timeout here, else this hangs if backup not reachable

	if err != nil {
		fmt.Printf("Node %d: Failed to connect to backup %s: %v", r.server.PID, addr_string, err)
		return err
	}

	var to_resp ReplicationResponse
	client := rpc.NewClient(caller)
	err = client.Call("ReplicationHandler.ApplyEntries", req, &to_resp)
	client.Close()

	if err != nil {
		log.Printf("Node %d: Failed to replicate to %s: %v", r.server.PID, addr_string, err)
		return err
	} else if !to_resp.Success {
		log.Printf("Node %d: Replication to %s failed: %s", r.server.PID, addr_string, to_resp.Message)
		return err
	}
	//}
	resp.ID = -1
	return nil
}