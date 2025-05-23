import { useState, useEffect } from 'react'
import { BACK_END_PORT, CONTACTS_ROUTE, ALL_USERS_ROUTE, ADD_CONTACT_ROUTE } from '../const'
import { useGlobal } from '../globalContext';
import { PlusIcon, XMarkIcon } from "@heroicons/react/24/outline";

/**
 * ChatList React component
 * 
 * Manages scrollable contact view on left side of home screen
 * 
 * 
 */

/**
 *  ChatContact card, item type in list on left side of screen
 * @param {*} props 
 * @returns 
 */
export const ChatContact = (props) => {
    return (
        <li key={props.email} className="flex justify-between gap-x-6 p-5 border-solid border-b-1 ">
            <div className="flex min-w-0 gap-x-4">


                <div className="min-w-0 flex-auto">
                    <p className="text-sm/6 font-semibold text-gray-900">{props.name}</p>
                    <p className="mt-1 truncate text-xs/5 text-gray-500">{props.email}</p>
                </div>
            </div>
            <div className="hidden shrink-0 sm:flex sm:flex-col sm:items-end">
                <p className="text-sm/6 text-gray-900">{props.role}</p>
            </div>
        </li>
    );
}


/**
 * How a user's profile appears when adding contacts
 * @param {*} props 
 * @returns 
 */
export const NameListItem = (props) => {
    const { userProfile, setUserProfile } = useGlobal()

    /**
     * Clicking user's name adds contact
     * 
     */
    const sendCreateContactMessage = () => {
        var req_body = {
            UserId: userProfile.UserId,
            ContactId: props.UserId
        }
        const options = {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify(req_body),
        };

        // HTTP to back as usual
        fetch(`http://127.0.0.1:${BACK_END_PORT}/${ADD_CONTACT_ROUTE}`, options)
            .then(response => {
                if (!response.ok) {
                    alert("Error adding friend. You may already have them as a friend. Or try again later")
                }
                else {
                    alert("Friend added!")
                }

            })

        props.refreshFunction();
    }

    return (
        <button onClick={sendCreateContactMessage} className='w-full text-left hover:bg-gray-100 active:bg-white'>
            <div>
                <p className="grid grid-cols-1 text-gray-800 text-xl font-bold font-sans ">
                    {props.Firstname} {props.Lastname}
                </p>
                <p className="paddingtext-gray-800 text-sm font-sans  ">
                    {props.Email}
                </p>
            </div>
        </button>
    );


}

/**
 * Container, list of available contacts to add, item type is NameListItem
 * @param {*} props 
 * @returns 
 */
export const NameList = (props) => {

    const { allUsers, setAllUsers } = useGlobal();      // all users in system (proof of concept, pulled at page load)
    const [usersToDisplay, setUsersToDisplay] = useState([])   
    const [currentInput, setCurrentInput] = useState("");

    // filter names in list based on input text content
    const handleInputChange = (e) => {
        setCurrentInput(e.target.value);
        if (e.target.value === "") {
            setUsersToDisplay([])
        }
        else {
            setUsersToDisplay(allUsers.filter((user) => `${user.Firstname}${user.Lastname}${user.Email}`.toLowerCase().includes(e.target.value.toLowerCase())))
        }
    }


    const listDiv = (
        <div className="fixed left-1/2 top-1/2 transform bg-white -translate-x-1/2 -translate-y-1/2 shadow-md rounded-xl h-[36rem] w-80 max-w-96 pb-16">

            <div className="grid grid-cols-2 flex justify-center p-4">
                <p className="paddingtext-gray-800 text-4xl font-sans font-bold ">
                    Add
                </p>

                <button onClick={props.buttonAction} className="h-10 w-10 text-white bg-gradient-to-b from-red-400 to-red-500 rounded-full flex items-center justify-center">
                    <XMarkIcon className="w-6 h-6" />
                </button>
                <form onChange={handleInputChange} onSubmit={x => { }}>
                    <div class="grid grid-cols-2">
                        <input type="text"
                            id="first_name"
                            className="w-[100%] border border-gray-300 mt-2 p-2 h-10 bg-gray-white text-gray-600 rounded-3xl w-72"
                            placeholder="Search Users"
                            required
                            value={currentInput}
                            autoComplete="off" />

                    </div>

                </form>
            </div>

            <ul role="list"
                className="overflow-auto h-[calc(100%-64px)] px-4 py-2 w-full flex flex-col gap-y-4">


                {usersToDisplay.map((item, index) => (
                    // filtered list of NameListItems
                    <NameListItem
                        key={index}
                        refreshFunction={props.refreshFunction}
                        Firstname={item.Firstname}
                        Lastname={item.Lastname}
                        Email={item.Email}
                        UserId={item.UserId}
                    />
                ))}

            </ul>


        </div>
    )
    return (
        props.nameListVisible ? listDiv : null
    );

}


/**
 * Main component for chat/contact list on left side of screen
 * 
 * @returns 
 * 
 */
export default function ChatList() {

    const { userProfile, setUserProfile } = useGlobal() // current user profile
    const [contacts, setContacts] = useState(null); // user's contacts
    const [searchContacts, setSearchContacts] = useState([]);   // contacts filtered based on text input
    const [nameListVisible, setNameListVisible] = useState(false);  // are we adding a contact?
    const { selectedContactId, setSelectedContactId } = useGlobal();    // selected chat id
    const [chatSearchText, setChatSearchText] = useState("")    // text box, searching through chats
    const { allUsers, setAllUsers } = useGlobal();  // all users from backend (Proof of concept)

    
    // need to continuously check for use
    useEffect(() => {
        const interval = setInterval(() => {    // ask for latest users and contacts silently every second
            GetContacts();
            getAllUsers();
        }, 1000);
        return () => clearInterval(interval);
    }, []);



    const handleInputChange = (e) => {
        setChatSearchText(e.target.value);
        setSearchContacts(contacts.filter((user) => `${user.Firstname}${user.Lastname}${user.Email}`.toLowerCase().includes(e.target.value.toLowerCase())))
    }


    const closeNameList = () => {
        setNameListVisible(false);
    }

    /**
     * Request all users from backend
     * 
     */
    const getAllUsers = () => {
        var req_body = {
            UserId: userProfile.UserId,
        }
        const options = {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify(req_body),
        };
        fetch(`http://127.0.0.1:${BACK_END_PORT}/${ALL_USERS_ROUTE}`, options)
            .then(response => {
                if (!response.ok) {
                    alert("Error logging in. Try different email/password or please try again later")
                    return "{}"
                }
                else {
                    return response.text()
                }
            })
            .then(data => {
                const parsedData = JSON.parse(data)
                setAllUsers(parsedData ? parsedData : [])
            })

    }

    /**
     * get user's registered contacts from backend
     * 
     */
    const GetContacts = () => {
        var req_body = {
            UserId: userProfile.UserId,
        }
        const options = {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify(req_body),
        };
        fetch(`http://127.0.0.1:${BACK_END_PORT}/${CONTACTS_ROUTE}`, options)
            .then(response => {
                if (!response.ok) {
                    alert("Error logging in. Try different email/password or please try again later")
                    return "{}"
                }
                else {
                    return response.text()
                }
            })
            .then(data => {
                const parsedData = JSON.parse(data)
                setContacts(parsedData ? parsedData : [])
            })

    }




    return (
        <div className="w-2/4 h-4/4">
            <NameList refreshFunction={GetContacts} nameListVisible={nameListVisible} buttonAction={closeNameList} />
            <text className="paddingtext-gray-800 text-4xl font-sans font-bold ">
                Chats
            </text>
            <div class="grid grid-cols-2">
                <button onClick={setNameListVisible} className="flex items-center justify-center ml-2 mt-2 h-10 w-[25%] w-16 text-white bg-gradient-to-b from-blue-400 to-blue-500 rounded-3xl">
                    <PlusIcon className="w-6 h-6" />
                </button>
                <form onChange={handleInputChange} onSubmit={x => { }}>

                    <input type="text"
                        id="first_name"
                        className="w-[100%] border border-gray-300 mt-2 p-2 h-10 bg-gray-white text-gray-600 rounded-3xl w-74"
                        placeholder="Search Users"
                        required
                        value={chatSearchText}
                        autoComplete="off" />
                </form>
            </div>



            {contacts === null ? (
                <p>Loading...</p>
            ) : contacts.length === 0 ? (
                [<p>No contacts found</p>]
            ) : (
                <ul role="list" className="divide-y divide-gray-100 border-solid shadow-md rounded-xl h-96 overflow-auto">
                    {
                        searchContacts.map((person) => (
                            <button className="block w-full text-left hover:bg-gray-100 bg-transparent border-0 p-0 m-0 outline-none focus:outline-none"
                                onClick={() => setSelectedContactId(person.UserId)}>
                                <ChatContact
                                    userid={person.UserId}
                                    name={`${person.Firstname} ${person.Lastname}`}
                                    email={person.Email}
                                    role={person.Descr}
                                    lastSeen={""}
                                    lastSeenDateTime={""}

                                />
                            </button>
                        ))
                
                    }
                </ul>
            )}

        </div>
    );
}
