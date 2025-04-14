package server
/*
	Source code that encapsulates leader-election
	and fault tolerance logic

	The system, at any given point in time, maintains
	a single leader replica to receive client requests.

	The system uses a bully algorithm to ensure
	that the currently active replica with the highest ID
	is always the sole leader

*/

import (
	"fmt"
	_ "modernc.org/sqlite"
	"net"
	"net/rpc"
	"time"
)

/*
	This function is executed by the heartbeat monitor thread

	The purpose of this message is to ask the leader if they are ok

	If they receive an aknowledgement, true is returned (no elect)

	Else false is returned, and the calling code should call an election
*/
func (r *ReplicationHandler) BullyFailureDetector() bool {
	if r.server.PID == r.server.LeaderID {
		// Leader can do a time and log sync here instead of detecting leader failure
		r.SyncTime()
		r.SyncLogs()
		return false // leader can't detect its own failures, return false
	}

	leader_addr := r.server.BackupNodes[r.server.LeaderID]
	addr_string := net.JoinHostPort(leader_addr.Address, fmt.Sprintf("%d", leader_addr.Port))

	// formula from class for expected time
	t_trans := 100 * time.Millisecond // upper bound
	t_proc := 25 * time.Millisecond   // upper bound
	t := (2 * t_trans) + t_proc

	// dial leader replica with timreout specified
	caller, err := net.DialTimeout("tcp", addr_string, t) // need a timeout here, else this hangs if backup not reachable
	if err != nil {
		fmt.Printf("Leader down: %s\n", err)
		return true
	}

	// invoke rpc
	var resp ReplicationResponse
	var msg ReplicationRequest
	client := rpc.NewClient(caller)
	err = client.Call("ReplicationHandler.IsStatusOK", msg, &resp)
	if err != nil {
		fmt.Printf("Leader down: %s\n", err)
		return true
	}

	// handle response
	if resp.Message != "STATUSOK" {
		fmt.Printf("Leader down: %s\n", err)
		return true
	}
	return false
}

/*
	This function will return the ID of a replica, if invoked
	via RPC
*/
func (r *MessageHandler) GetPID(msg *IDNumber, resp *IDNumber) error {
	resp.ID = r.server.PID
	return nil
}

/*
	A wrapper function to make sending bully messages simpler
	
	Replica address, RPC function name, and type of bully message (LEADER / ELECTION)
	must be specified
*/
func SendBullyMessage(replica ReplicaAddress, funcName string, msg BullyMessage, resp *ReplicationResponse) error {
	addr_string := net.JoinHostPort(replica.Address, fmt.Sprintf("%d", replica.Port))

	// try connection to replica address for
	// rpc call
	caller, err := net.DialTimeout("tcp", addr_string, 1*time.Second) 
	if err != nil {
		fmt.Printf("Replica at %s is offline\n", addr_string)
		return err
	}

	// invoke RPC, funcName specifies which sort of message we would like to send
	client := rpc.NewClient(caller)
	err = client.Call(fmt.Sprintf("ReplicationHandler.%s", funcName), msg, resp) 
	return err
}


/*
	Main Bully Algorithm election algorithm

	Derived from pseudocode provided in class

*/
func (r *ReplicationHandler) InitiateElection() bool {
	r.server.Running = true

	// debug
	fmt.Println("CALLING ELECTION")

	// if I am max ID, then I must be the new leader!
	if r.server.PID == len(r.server.BackupNodes)-1 {
		for _, replica := range r.server.BackupNodes {
			if IsAddressSelf(r.server.AddressPort, replica) { // skip crashed leader and self
				continue
			}

			// bully others via RPC
			var resp ReplicationResponse
			msg := BullyMessage{PID: r.server.PID, Message: "LEADER"}
			r.server.LeaderID = r.server.PID
			r.server.Running = false
			SendBullyMessage(replica, "BullyLeader", msg, &resp)

			// I won, send my address to cached client addresses
			r.server.SendLeaderAddressToClients()
		}
	} else {
		// otherwise, let's see if any higher PIDs are above me
		electionResponse := ReplicationResponse{LastIndex: -1}
		for j, replica := range r.server.BackupNodes {
			if j <= r.server.PID { // skip crashed leader and self
				continue
			}

			// try sending election to others
			msg := BullyMessage{PID: r.server.PID, Message: "ELECTION"}
			SendBullyMessage(replica, "BullyElection", msg, &electionResponse)

		}

			// formula from class for expected time
		t_trans := 100 * time.Millisecond // upper bound
		t_proc := 25 * time.Millisecond   // upper bound
		t := (2 * t_trans) + t_proc

		time.Sleep(t)

		// no responses, send leader to all other active replicas
		if electionResponse.LastIndex == -1 {
			r.server.LeaderID = r.server.PID
			for j, replica := range r.server.BackupNodes {
				// skip self
				if j == r.server.PID {
					continue
				}
				msg := BullyMessage{PID: r.server.PID, Message: "LEADER"}
				SendBullyMessage(replica, "BullyLeader", msg, nil)
			}

			// I won, send my address to cached client addresses
			r.server.SendLeaderAddressToClients()

		} else {
			time.Sleep(t)
			r.server.Running = false
		}

	}
	return false
}


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
