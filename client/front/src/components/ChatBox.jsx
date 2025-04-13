import { useEffect, useState, useRef, forwardRef } from "react";
import { Description, Field, Label, Textarea } from '@headlessui/react'
import { ArrowUpRightIcon, PlusIcon } from "@heroicons/react/24/outline";
import {BACK_END_PORT, INCOMING_ROUTE, MESSAGES_ROUTE} from '../const';
import clsx from 'clsx'
import { GlobalProvider, useGlobal } from "../globalContext";


/**
 * ChatBox React component
 * 
 * Manages scrollable chat window on right side of main screen
 * 
 * 
 */


/**
 * Chat bubble
 * 
 */
export const ChatBubble = forwardRef((props, ref) => {


    // styles depending on sender vs receiver
    const class_recv = "m-4 grid grid-cols-1 p-2 rounded-xl bg-gray-100 text-gray-700 w-fit max-w-[50%] justify-self-start";
    const class_send = "m-4 grid grid-cols-1 p-2 rounded-xl bg-gradient-to-b from-blue-400 to-blue-500 text-white w-fit max-w-[50%] justify-self-end";

    return (   
        // colored / styled div based on sender vs receiver
        <div className="grid grid-cols-1" ref={ref}>
            <div className={props.recv === true ? class_recv : class_send}>
                <text className="font-sans text-base">
                    {props.msg}
                </text>
                <text className="font-sans text-xs mt-2">
                    {props.timestamp}
                </text>
            </div>
        </div>
    );
})



/**
 * Manages actual chatbox
 * 
 * @returns 
 */
export default function ChatBox() {

    const {renderedMessages, setRenderedMessages} = useGlobal([])   // messaged to show
    const {selectedContactId, setSelectedContactId} = useGlobal();  // chat id
    const [currentInput, setCurrentInput] = useState("");   // text in input
    const {userProfile, setUserProfile} = useGlobal()   // global user profiled
    const latestMessageCount = useRef(0);

    const latestMessage = useRef(null);

    useEffect(() => {
        if (selectedContactId !== -1) {
            GetMessages();
    
            const interval = setInterval(() => {    // ping for latest messages every second
                GetMessages();
            }, 1000); 
    
            return () => clearInterval(interval);
        }
    
        if (latestMessage.current) {
            latestMessage.current.scrollIntoView({ behavior: "smooth" });
        }
    }, [selectedContactId]);


    /**
     * Scroll down to latest message if new received
     */
    useEffect(() => {
      //  alert(latestMessageCount);
        //alert(renderedMessages.length)
        if (latestMessageCount.current !== renderedMessages.length) {
            latestMessageCount.current = renderedMessages.length;
            if (latestMessage.current) {
                latestMessage.current.scrollIntoView({ behavior: "smooth" });
            }
        }
    }, [renderedMessages]);



    /**
     * Request messages from backend
     */
    const GetMessages = () => {
        var req_body = {
            UserId: userProfile.UserId, // send my userid and selected contact
            ContactId: selectedContactId,
        }

        const options = {       // 
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify(req_body),
        };
        // HTTP, localhost to client process
        fetch(`http://127.0.0.1:${BACK_END_PORT}/${MESSAGES_ROUTE}`, options)
        .then(response => {
        if (!response.ok) {
            alert("Error getting messages")
            return "{}"
        }
        else {
            return response.text()
        }
        })  // result
        .then(data => {
            if (JSON.parse(data) !== null) {  // screw it no messages for now I guess
                setRenderedMessages(JSON.parse(data))
            }
            else {
                setRenderedMessages([])
            }
        })
        
    }


    const handleInputChange = (e) => {
        setCurrentInput(e.target.value);
    }


    /**
     *  Send message to backend, set JSON object and HTTP over localhost
     * @param {*} _msg 
     */
    const sendInputToBack = (_msg) => {
        var req_body = {
            From: _msg.From,
            To: _msg.To,
            Message: _msg.Message,
            Timestamp: _msg.Timestamp,
            Acked: 1
        }
        const options = {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify(req_body),
        };
        fetch(`http://127.0.0.1:${BACK_END_PORT}/${INCOMING_ROUTE}`, options)
    }


    
    /**
     * Send message to backend on form submit
     * @param {*} e 
     */
    const handleSubmit = (e) => {
        e.preventDefault();
        var now = new Date();
        var to_send = {From: userProfile.UserId,
            To: selectedContactId,
            Message: currentInput, 
            Timestamp: `${now.getHours()}:${new String(now.getMinutes()).padStart(2, "0")}`, 
            Acked: 1
        }
        sendInputToBack(to_send);
        setCurrentInput("");
        setRenderedMessages(prev => [...prev, to_send]);


    }


    return (
        <div className="w-4/4">
            <text className="paddingtext-gray-800 text-4xl font-sans font-bold ">
                Messages
            </text>
            <ul role="list" className=" shadow-md rounded-xl h-96 overflow-auto">
                {renderedMessages.map((_msg, index) => (
                    // list of chatbubbles formatted with messages from backend
                    <li key={index}>
                        <ChatBubble  ref={index === renderedMessages.length - 1 ? latestMessage : null}
                         msg={_msg.Message} timestamp={_msg.Timestamp} recv={_msg.From !== userProfile.UserId} />
                    </li>
                ))}
            </ul>
            
            
            <form onSubmit={handleSubmit}>
            <div class="grid grid-cols-2">
                <input onChange={handleInputChange} type="text" 
                id="first_name" 
                className="w-[100%] border border-gray-300 mt-2 p-2 h-10 bg-gray-white text-gray-600 rounded-3xl" 
                placeholder="Text Message" 
                required 
                value={currentInput}
                autoComplete="off"/>
                <button type="submit" className="flex items-center justify-center ml-2 mt-2 h-10 w-[25%] text-white bg-gradient-to-b from-blue-400 to-blue-500 rounded-3xl">
                    <ArrowUpRightIcon className="w-6 h-6" />
                </button>
            </div>

        </form>

        </div>
    );



}