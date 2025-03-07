
import ChatList from '../components/ChatList';
import Navbar from '../components/Navbar';
import ChatBox from '../components/ChatBox';



export default function HomePage() {


    return (
        <div>

            <Navbar />
            <div class="grid grid-cols-2 gap-4 m-32">




                <ChatList />
                <ChatBox/>
            </div>
        </div>
    );



}