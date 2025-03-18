package main

import (
	"fmt"
	"time"
)

func main() {
	// Configuration
	basePort := 9999
	numServers := 3 // 3 Nodes for now

	// Create an array to hold server references
	var servers []*Server

	// Start the leader first (PID 0)
	leader := spawn_server(0, basePort)
	servers = append(servers, leader)

	// Give the leader time to initialize
	time.Sleep(1 * time.Second)

	// Keep track of backup addresses for the leader
	backupAddresses := make([]string, 0)

	// Start backup nodes
	for i := 1; i < numServers; i++ {
		port := basePort + i
		backupAddr := fmt.Sprintf("%s%d", RPC_ADDRESS, port)
		backupAddresses = append(backupAddresses, backupAddr)

		backup := spawn_server(i, port)
		servers = append(servers, backup)
	}

	// Let the leader know about the backup nodes
	time.Sleep(1 * time.Second)
	leader.SetBackupNodes(backupAddresses)

	fmt.Println("Clients may now connect")

	// Wait forever
	select {}
}
