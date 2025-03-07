
import ChatList from '../components/ChatList';
import Navbar from '../components/Navbar';



export default function HomePage() {


    return (
        <div>

            <Navbar />
            <div class="grid grid-cols-2 gap-4 m-32">




                <ChatList />
            </div>
        </div>
    );



}