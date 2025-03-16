
import ChatList from '../components/ChatList';
import Navbar from '../components/Navbar';
import ChatBox from '../components/ChatBox';
import { GlobalProvider, useGlobal } from '../globalContext';


export default function HomePage() {
    const {userProfile} = useGlobal()

    const loggedin = () => {
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

    const loggedout = () => {
        return (
        <div>
            <p>
            Error 404: Page Not Found  
            </p>
        </div>
        );
    }


    return (
        Object.keys(userProfile).length === 0 ? loggedout() : loggedin()
    );



}