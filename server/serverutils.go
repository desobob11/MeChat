package main

import (
	// "bufio"
	"fmt"
	"strconv"
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

type UserProfile struct {
	UserId int
	Email string
	Firstname string
	Lastname string
	Descr string
}

type GetMessagesRequest struct {
	UserId int
	ContactId int
}

type Contacts struct {
	ContactList []UserProfile
}

type MessageList struct {
	Messages []ChatMessage
}


type LoginMessage struct {
	Email string
	Password string
}

type RPCResponse struct {
	Message string
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
		*response = "error"
		return err
	}

	fmt.Print("Wrote message")

	*response = "ACK"
	return nil
}


func (t* MessageHandler) CreateAccount(message *CreateAccountMessage, response *RPCResponse) error {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	script := `INSERT INTO users (
		[password], 
		[email], 
		[firstname], 
		[lastname], 
		[descr])
	VALUES (?, ?, ?, ?, ?);`

	
	result, err := _db.Exec(script, message.Password,
		 message.Email,
		  message.Firstname,
		   message.Lastname,
		    message.Descr)

	if err != nil {
		fmt.Println("Error creating user. ")		// should print out rows changed here eventually
		fmt.Println(err)
		response.Message = "error"
		return err
	}

	uid, err := result.LastInsertId()

	if err != nil {
		fmt.Println("Error getting user id")
		fmt.Println(err)
		response.Message = "error"
		return err
	}

	
	uid_str := strconv.Itoa(int(uid))

	fmt.Println("Created user")

	response.Message = uid_str
	return nil
}


func (t* MessageHandler) Login(message *LoginMessage, user_profile *UserProfile) error {
	t.mutex.Lock()
	defer t.mutex.Unlock()



	pass_check := `SELECT  
	[password]
	FROM users
	WHERE email = ?`

	pass_row, err := _db.Query(pass_check, message.Email)
	if err != nil {
		fmt.Println("Error checking password")
		return err
	}

	if !pass_row.Next() {
		fmt.Println("No such user")
		return fmt.Errorf("no such user")
	}

	for pass_row.Next() {
		var db_pass string
		err = pass_row.Scan(&db_pass);
		fmt.Println(db_pass)
		fmt.Println(message.Password)
		if err != nil {
			fmt.Println("Error scanning password")
			return err
		}

		if db_pass != message.Password {
			fmt.Println("Incorrect password")
			return fmt.Errorf("incorrect password")
		}
	}
	pass_row.Close()
	

	query := `SELECT  
		[userid],
		[email], 
		[firstname], 
		[lastname], 
		[descr]
		FROM users
		WHERE email = ?`

	user_row, err := _db.Query(query, message.Email)
	if err != nil {
		fmt.Println(err)
		return err
	}

	for user_row.Next() {

		err = user_row.Scan(&user_profile.UserId, &user_profile.Email, &user_profile.Firstname, &user_profile.Lastname, &user_profile.Descr);
		if err != nil {
			fmt.Println(err)
			return err
		}
	}
	user_row.Close()

	fmt.Println("User profile fetched")
	fmt.Println(user_profile.Email)
	fmt.Println(user_profile.Descr)
	fmt.Println(user_profile.Firstname)
	return nil
}


func (t* MessageHandler) GetContacts(message *UserProfile, contacts *Contacts) error {
	t.mutex.Lock()
	defer t.mutex.Unlock()


		query := ` SELECT
						U.userid,
						U.email,
						U.firstname,
						U.lastname,
						U.descr

						FROM contacts C

						INNER JOIN users U
						ON U.userid = C.contactid

						WHERE C.userid = ?`


	rows, err := _db.Query(query, message.UserId)
	if err != nil {
		fmt.Println(err)
		return err
	}

	for rows.Next() {
		var contact UserProfile
		err = rows.Scan(&contact.UserId, &contact.Email, &contact.Firstname, &contact.Lastname, &contact.Descr);

		if err != nil {
			fmt.Println(err)
			return err
		}

		contacts.ContactList = append(contacts.ContactList, contact)
	}
	rows.Close()


	fmt.Println("Contacts fetched")

	return nil
}


func (t* MessageHandler) GetMessages(message *GetMessagesRequest, messages *MessageList) error {
	t.mutex.Lock()
	defer t.mutex.Unlock()


	query := 
			`SELECT
			M.from_userid,
			M.to_userid,
			M.message,
			M.timestamp,
			M.acked

			FROM messages M


			WHERE (M.from_userid = ?
			AND M.to_userid =  ?)
			OR
			(M.from_userid = ?
   			 AND M.to_userid =  ?)`

	rows, err := _db.Query(query, message.UserId, message.ContactId, message.ContactId, message.UserId)
	if err != nil {
		fmt.Println(err)
		return err
	}

	for rows.Next() {
		var msg ChatMessage
		err = rows.Scan(&msg.From, &msg.To, &msg.Message, &msg.Timestamp, &msg.Acked);

		if err != nil {
			fmt.Println(err)
			return err
		}

		messages.Messages = append(messages.Messages, msg)
	}
	rows.Close()


	fmt.Println("Messages fetched")

	return nil
}










func BuildDatabase() (*sql.DB, error) {
	users_script := `CREATE TABLE users (
						userid INTEGER PRIMARY KEY,
						password TEXT, 
						email TEXT UNIQUE, 
						firstname TEXT, 
						lastname TEXT, 
						descr TEXT);`

	contacts_script :=`CREATE TABLE contacts (
						rec_id INTEGER PRIMARY KEY,
						userid INTEGER, 
						contactid INTEGER);`

	messages_script :=`CREATE TABLE messages (
						rec_id INTEGER PRIMARY KEY,
						from_userid INTEGER, 
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