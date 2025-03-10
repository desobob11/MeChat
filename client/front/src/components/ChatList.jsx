import {useState, useEffect} from 'react'
import {BACK_END_PORT, CONTACTS_ROUTE} from '../const'
import {useGlobal } from '../globalContext';


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



export default function ChatList() {

    const {userProfile, setUserProfile} = useGlobal()

    const {selectedContactId, setSelectedContactId} = useGlobal()
    
    const [contacts, setContacts] = useState([])


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
            setContacts(JSON.parse(data))
            })
            
        }


    const SetContact = () => {

    }

    useEffect(() => {
        if (contacts.length === 0) {
           GetContacts();
        }

    }, [contacts])


    return (
        <div className="w-2/4 h-4/4">

            <text className="paddingtext-gray-800 text-4xl font-sans font-bold ">
                Chats
            </text>
            <ul role="list" className="divide-y divide-gray-100 border-solid shadow-md rounded-xl h-96 overflow-auto">
                {contacts.map((person) => (
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
                    
                ))}
            </ul>
        </div>
    );
}
