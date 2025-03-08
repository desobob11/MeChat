
import ChatList from '../components/ChatList';
import Navbar from '../components/Navbar';
import ChatBox from '../components/ChatBox';



export default function HomePage() {


    return (
        <div>

            <Navbar />
            <div class="pl-[5%] grid grid-cols-2 gap-1 m-32">




                <ChatList />
                <ChatBox/>
            </div>
        </div>
    );



}