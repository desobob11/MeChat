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

type CreateAccountMessage struct {
    Email string
    Password string
    Firstname string         // email = key?
    Lastname string           // email = key?
	Descr string			// bool 0 | 1
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

	*response = "ACK"
	return nil
}


func (t* MessageHandler) CreateAccount(message *CreateAccountMessage, response *string) error {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	script := `INSERT INTO users (
		[password], 
		[email], 
		[firstname], 
		[lastname], 
		[descr])
	VALUES (?, ?, ?, ?, ?);`

	
	_, err := _db.Exec(script, message.Password,
		 message.Email,
		  message.Firstname,
		   message.Lastname,
		    message.Descr)
	if err != nil {
		fmt.Println("Error creating user. ")		// should print out rows changed here eventually
		fmt.Println(err)
		*response = err.Error()
		return err
	}

	fmt.Print("Created user")

	*response = "ACK"
	return nil
}












func BuildDatabase() (*sql.DB, error) {
	users_script := `CREATE TABLE users (
						userid INTEGER PRIMARY KEY,
						password TEXT, 
						email TEXT, 
						firstname TEXT, 
						lastname TEXT, 
						descr TEXT);`

	contacts_script :=`CREATE TABLE contacts (
						userid INTEGER PRIMARY KEY, 
						contactid INTEGER);`

	messages_script :=`CREATE TABLE messages (
						from_userid INTEGER PRIMARY KEY, 
						to_userid INTEGER,
						message TEXT,
						timestamp TEXT,
						acked INTEGER);`			// bool



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