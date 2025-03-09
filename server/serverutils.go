package main

import (
	// "bufio"
	"fmt"
	"sync"
	// "log"
	// "net"
	// "strings"
	//  "net/rpc"
	"database/sql"


	_ "modernc.org/sqlite"
)


 const DB_NAME = "mechat.sqlite"
 const RPC_ADDRESS = "127.0.0.1:9999"


 type ChatMessage struct {
    Message string
    Timestamp string
    From int         // email = key?
    To int           // email = key?
	Acked int			// bool 0 | 1
}

type MessageHandler struct {
	mutex sync.Mutex
}


/*
	Saves record of this message trying to be sent from user
*/
func (t* MessageHandler) SaveMessage(message *ChatMessage, response *string) error {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	script := `INSERT INTO messages (
	[from_userid], 
	[to_userid], 
	[message], 
	[timestamp], 
	[acked]) 
	VALUES (?, ?, ?, ?, ?);`

	
	_, err := _db.Exec(script, message.From,
		 message.To,
		  message.Message,
		   message.Timestamp,
		    message.Acked)
	if err != nil {
		fmt.Println("Error saving message. ")		// should print out rows changed here eventually
		fmt.Println(err)
		*response = err.Error()
		return err
	}

	fmt.Print("Wrote message")

	*response = ""
	return nil
}












func BuildDatabase() (*sql.DB, error) {
	users_script := `CREATE TABLE users (
						userid INT, 
						email TEXT, 
						firstname TEXT, 
						lastname TEXT, 
						descr TEXT);`

	contacts_script :=`CREATE TABLE contacts (
						userid INT, 
						contactid INT);`

	messages_script :=`CREATE TABLE messages (
						from_userid INT, 
						to_userid INT,
						message TEXT,
						timestamp TEXT,
						acked INT);`			// bool



	db, err := sql.Open("sqlite", DB_NAME)
	if err != nil {
		fmt.Println("Error creating database file.")
		return nil, err;
	}

	_, err = db.Exec(users_script)
	if err != nil {
		fmt.Println("Error creating users table. ")
		return nil, err;
	}

	_, err = db.Exec(contacts_script)
	if err != nil {
		fmt.Println("Error creating contacts table. ")
		return nil, err;
	}

	_, err = db.Exec(messages_script)
	if err != nil {
		fmt.Println("Error creating messages table. ")
		return nil, err;
	}
	return db, nil	// successful
}