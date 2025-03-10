import { useEffect, useState, useRef, forwardRef } from "react";
import { Description, Field, Label, Textarea } from '@headlessui/react'
import { ArrowUpRightIcon } from "@heroicons/react/24/outline";
import {BACK_END_PORT, INCOMING_ROUTE} from '../const';
import clsx from 'clsx'




export const ChatBubble = forwardRef((props, ref) => {
    const class_recv = "m-4 grid grid-cols-1 p-2 rounded-xl bg-gray-100 text-gray-700 w-fit max-w-[50%] justify-self-start";
    const class_send = "m-4 grid grid-cols-1 p-2 rounded-xl bg-gradient-to-b from-blue-400 to-blue-500 text-white w-fit max-w-[50%] justify-self-end";

    return (
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

class Message {
    constructor(from, to, msg, timestamp, acked) {
        this.from = from;
        this.to = to;
        this.msg = msg;
        this.timestamp = timestamp;
        this.acked = acked;
    }
}




export default function ChatBox() {

    const {renderedMessages, setRenderedMessages} = useGlobal([])



    const [currentInput, setCurrentInput] = useState("");

    useEffect(() => {
        if (latestMessage.current) {
            latestMessage.current.scrollIntoView({ behavior: "smooth" });
        }
    }, [msgHistory])

    const latestMessage = useRef(null);

    const handleInputChange = (e) => {
        setCurrentInput(e.target.value);
    }


    const sendInputToBack = (_msg) => {
        var req_body = {
            from: _msg.from,
            to: _msg.to,
            msg: _msg.msg,
            timestamp: _msg.timestamp,
            acked: 0
        }
        const options = {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify(req_body),
        };
        fetch(`http://127.0.0.1:${BACK_END_PORT}/${INCOMING_ROUTE}`, options)
    }


    

    const handleSubmit = (e) => {
        e.preventDefault();
        var now = new Date();
        var to_send = new Message(
            0,
            1,
            currentInput, 
            `${now.getHours()}:${new String(now.getMinutes()).padStart(2, "0")}`, 
            false
        );
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
                    <li key={index}>
                        <ChatBubble  ref={index === msgHistory.length - 1 ? latestMessage : null}
                         msg={_msg.msg} timestamp={_msg.timestamp} recv={_msg.recv} />
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