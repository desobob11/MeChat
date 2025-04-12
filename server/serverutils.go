package main

/*
	Secondary back-end server source file.
	
	Contains logic for accessing SQL DB,
	RPC interfaces for client-server communication

*/

import (
	"fmt"
	"strconv"
	"database/sql"
	_ "modernc.org/sqlite"
)

const RPC_ADDRESS = "127.0.0.1:" // leave space for the port number

// JSON object, represents chat message receieved by user
type ChatMessage struct {
	Message   string
	Timestamp string
	From      int
	To        int
	Acked     int
}

// JSON object, represents create account request received from user
type CreateAccountMessage struct {
	Email     string
	Password  string
	Firstname string
	Lastname  string
	Descr     string
}

// JSON object, represents details that are relevant to a user, regaring
// other user profiles
type UserProfile struct {
	UserId    int
	Email     string
	Firstname string
	Lastname  string
	Descr     string
}

// JSON object, represents a chat between two users. Used
// for querying sent chat messages
type GetMessagesRequest struct {
	UserId    int
	ContactId int
}

// JSON object, array of user profiles
type Contacts struct {
	ContactList []UserProfile
}

// JSON object, arrat of Chat messages
type MessageList struct {
	Messages []ChatMessage
}

// JSON object, represents email and password provided by user
type LoginMessage struct {
	Email    string
	Password string
}

// JSON object, represents a specific RPC response from a remote
// message handler
type RPCResponse struct {
	Message string
}

// JSON object, represents a request to create a new chat between two users
type AddContactMessage struct {
	UserId    int
	ContactId int
}

// JSON object, user ID number, wrapping in struct is necessary
// for Golang RPC
type IDNumber struct {
	ID int
}

// Debug Function
type NodeInfo struct {
	NodeID   int  `json:"node_id"`
	IsLeader bool `json:"is_leader"`
}

type LeaderId struct {
	LeaderID int
}

func GenerateDatabaseName(PID int) string {
	return "mechat0.sqlite"
}

/*
Saves record of this message trying to be sent from user
*/
func (t *MessageHandler) SaveMessage(message *ChatMessage, response *string) error {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	// Do not write if we arent the leader
	if t.server.LeaderID != t.server.PID {
		*response = "not the leader node"
		return fmt.Errorf("not the leader node")
	}

	script := `INSERT INTO messages (
		[from_userid], 
		[to_userid], 
		[message], 
		[timestamp], 
		[acked]) 
		VALUES (?, ?, ?, ?, ?);`

	_, err := t.server.DB.Exec(script, message.From,
		message.To,
		message.Message,
		message.Timestamp,
		message.Acked)
	if err != nil {
		fmt.Println("Error saving message. ") // should print out rows changed here eventually
		fmt.Println(err)
		*response = "error"
		return err
	}

	// Create a log entry without index
	entry := LogEntry{
		SQL: script,
		Args: []any{
			message.From,
			message.To,
			message.Message,
			message.Timestamp,
			message.Acked,
		},
	}

	// Append to log and get updated entry with proper index
	updatedEntry, err := t.server.AppendToLog(entry)
	if err != nil {
		fmt.Printf("Error appending to log: %v", err)
		// Continue despite error - we already applied locally
	} else {
		// Replicate the updated entry with proper index
		go t.server.ReplicateToBackups(updatedEntry)
	}

	fmt.Println("Wrote message")

	*response = "ACK"
	return nil
}

func (t *MessageHandler) CreateAccount(message *CreateAccountMessage, response *RPCResponse) error {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	// Dont write if we are not the leader
	if !t.server.IsLeader {
		response.Message = "not the leader node"
		return fmt.Errorf("not the leader node")
	}

	script := `INSERT INTO users (
		[password], 
		[email], 
		[firstname], 
		[lastname], 
		[descr])
	VALUES (?, ?, ?, ?, ?);`

	result, err := t.server.DB.Exec(script, message.Password,
		message.Email,
		message.Firstname,
		message.Lastname,
		message.Descr)

	if err != nil {
		fmt.Println("Error creating user. ") // should print out rows changed here eventually
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

	// Create a log entry without index
	entry := LogEntry{
		SQL: script,
		Args: []any{
			message.Password,
			message.Email,
			message.Firstname,
			message.Lastname,
			message.Descr,
		},
	}

	// Append to log and get updated entry with proper index
	updatedEntry, err := t.server.AppendToLog(entry)
	if err != nil {
		fmt.Printf("Error appending to log: %v", err)
		// Continue despite error - we already applied locally
	} else {
		// Replicate the updated entry with proper index
		go t.server.ReplicateToBackups(updatedEntry)
	}

	uid_str := strconv.Itoa(int(uid))
	fmt.Println("Created user")

	response.Message = uid_str
	return nil
}

func (t *MessageHandler) Login(message *LoginMessage, user_profile *UserProfile) error {
	t.mutex.Lock()
	defer t.mutex.Unlock()


	pass_check := `SELECT [password] FROM users WHERE email = ?`
	pass_row, err := t.server.DB.Query(pass_check, message.Email)

	if err != nil {
		fmt.Println("Error checking password")
		return err
	}

	if !pass_row.Next() {
		fmt.Println("No such user")
		return fmt.Errorf("no such user")
	}

	var db_pass string
	err = pass_row.Scan(&db_pass)
	if err != nil {
		fmt.Println("Error scanning password")
		pass_row.Close()
		return err
	}

	if db_pass != message.Password {
		fmt.Println("Incorrect password")
		pass_row.Close()
		return fmt.Errorf("incorrect password")
	}

	pass_row.Close()

	query := `SELECT [userid], [email], [firstname], [lastname], [descr] FROM users WHERE email = ?`
	user_row, err := t.server.DB.Query(query, message.Email)
	if err != nil {
		fmt.Println(err)
		return err
	}

	if user_row.Next() {
		err = user_row.Scan(&user_profile.UserId, &user_profile.Email, &user_profile.Firstname, &user_profile.Lastname, &user_profile.Descr)
		if err != nil {
			fmt.Println(err)
			user_row.Close()
			return err
		}
	}
	user_row.Close()

		

//	fmt.Println("User profile fetched")

	return nil
}

func (t *MessageHandler) AddContact(message *AddContactMessage, response *AddContactMessage) error {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	check := `SELECT * FROM contacts WHERE userid=? AND contactid=?`


	check_one, err1 := t.server.DB.Query(check, message.ContactId, message.UserId)
	check_two, err2 := t.server.DB.Query(check, message.UserId, message.ContactId)

	if err1 != nil || err2 != nil {
		fmt.Println("Error querying contact")
		check_one.Close()
		check_two.Close()
		return fmt.Errorf("error querying contact")
	}

	if check_one.Next() || check_two.Next() {
		check_one.Close()
		check_two.Close()
		fmt.Println("Contact already exists")
		return fmt.Errorf("contact already exists")
	}

	check_one.Close()
	check_two.Close()

	tx, err := t.server.DB.Begin()
    if err != nil {
        fmt.Println("Error beginning transaction: ", err)
        return fmt.Errorf("error beginning transaction: ")
    }

	script := `INSERT INTO contacts
				(userid, contactid) VALUES (?, ?)`

	_, err1 = tx.Exec(script, message.UserId, message.ContactId);
	_, err2 = tx.Exec(script, message.ContactId, message.UserId);
	fmt.Printf("%d    %d\n", message.UserId, message.ContactId)
	if err1 != nil || err2 != nil {
		fmt.Println("Error creating contact")
		tx.Rollback()
		return fmt.Errorf("error creating contact")
	}

	err = tx.Commit()
	if err != nil {
        fmt.Println("Error committing: ", err)
        return fmt.Errorf("error committing: ")
    }

	// Create a log entry without index
	entry := LogEntry{
		SQL: script,
		Args: []any{
			message.UserId,
			message.ContactId,
		},
	}

	// Append to log and get updated entry with proper index
	updatedEntry, err := t.server.AppendToLog(entry)
	if err != nil {
		fmt.Printf("Error appending to log: %v", err)
		// Continue despite error - we already applied locally
	} else {
		// Replicate the updated entry with proper index
		go t.server.ReplicateToBackups(updatedEntry)
	}

	// need both versions
	entry = LogEntry{
		SQL: script,
		Args: []any{
			message.ContactId,
			message.UserId,
		},
	}

		// Append to log and get updated entry with proper index
	updatedEntry, err = t.server.AppendToLog(entry)
	if err != nil {
		fmt.Printf("Error appending to log: %v", err)
		// Continue despite error - we already applied locally
	} else {
		// Replicate the updated entry with proper index
		go t.server.ReplicateToBackups(updatedEntry)
	}


	return nil
}


func (t *MessageHandler) GetContacts(message *UserProfile, contacts *Contacts) error {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	query := `SELECT
                U.userid,
                U.email,
                U.firstname,
                U.lastname,
                U.descr
                FROM contacts C
                INNER JOIN users U
                ON U.userid = C.contactid
                WHERE C.userid = ?`

	rows, err := t.server.DB.Query(query, message.UserId)
	if err != nil {
		fmt.Println(err)
		return err
	}

	contacts.ContactList = []UserProfile{}
	for rows.Next() {
		var contact UserProfile
		err = rows.Scan(&contact.UserId, &contact.Email, &contact.Firstname, &contact.Lastname, &contact.Descr)
		if err != nil {
			fmt.Println(err)
			rows.Close()
			return err
		}
		contacts.ContactList = append(contacts.ContactList, contact)
	}
	rows.Close()


	return nil
}


func (t *MessageHandler) GetAllUsers(message *UserProfile, contacts *Contacts) error {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	query := `SELECT
                userid,
                email,
                firstname,
                lastname,
                descr
				FROM users`

	rows, err := t.server.DB.Query(query)
	if err != nil {
		fmt.Println(err)
		return err
	}

	contacts.ContactList = []UserProfile{}
	for rows.Next() {
		var contact UserProfile
		err = rows.Scan(&contact.UserId, &contact.Email, &contact.Firstname, &contact.Lastname, &contact.Descr)
		if err != nil {
			fmt.Println(err)
			rows.Close()
			return err
		}
		contacts.ContactList = append(contacts.ContactList, contact)
	}
	rows.Close()

	//fmt.Println("All users fetched")
	return nil
}

func (t *MessageHandler) GetMessages(message *GetMessagesRequest, messages *MessageList) error {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	query := `SELECT
            M.from_userid,
            M.to_userid,
            M.message,
            M.timestamp,
            M.acked
            FROM messages M
            WHERE (M.from_userid = ?
            AND M.to_userid = ?)
            OR
            (M.from_userid = ?
            AND M.to_userid = ?)`

	rows, err := t.server.DB.Query(query, message.UserId, message.ContactId, message.ContactId, message.UserId)
	if err != nil {
		fmt.Println(err)
		return err
	}

	messages.Messages = []ChatMessage{}
	for rows.Next() {
		var msg ChatMessage
		err = rows.Scan(&msg.From, &msg.To, &msg.Message, &msg.Timestamp, &msg.Acked)
		if err != nil {
			fmt.Println(err)
			rows.Close()
			return err
		}
		messages.Messages = append(messages.Messages, msg)
	}
	rows.Close()

	return nil
}

func BuildDatabase(database_name string) (*sql.DB, error) {
	users_script := `CREATE TABLE users (
                        userid INTEGER PRIMARY KEY,
                        password TEXT, 
                        email TEXT UNIQUE, 
                        firstname TEXT, 
                        lastname TEXT, 
                        descr TEXT);`

	contacts_script := `CREATE TABLE contacts (
                        rec_id INTEGER PRIMARY KEY,
                        userid INTEGER, 
                        contactid INTEGER);`

	messages_script := `CREATE TABLE messages (
                        rec_id INTEGER PRIMARY KEY,
                        from_userid INTEGER, 
                        to_userid INTEGER,
                        message TEXT,
                        timestamp TEXT,
                        acked INTEGER);`

	db, err := sql.Open("sqlite", database_name)
	if err != nil {
		fmt.Println("Error creating database file.")
		return nil, err
	}

	_, err = db.Exec(users_script)
	if err != nil {
		fmt.Println("Error creating users table. ")
		return nil, err
	}

	_, err = db.Exec(contacts_script)
	if err != nil {
		fmt.Println("Error creating contacts table. ")
		return nil, err
	}

	_, err = db.Exec(messages_script)
	if err != nil {
		fmt.Println("Error creating messages table. ")
		return nil, err
	}
	return db, nil
}
