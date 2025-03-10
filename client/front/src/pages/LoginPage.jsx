
import ChatList from '../components/ChatList';
import Navbar from '../components/Navbar';
import ChatBox from '../components/ChatBox';
import { Link, useNavigate } from 'react-router-dom';
import { useEffect, useState } from 'react';
import { LOGIN_ROUTE, BACK_END_PORT } from '../const';
import { GlobalProvider, useGlobal } from '../globalContext';


class LoginMessage {
  constructor(pass, email) {
    this.email = email;
    this.pass = pass;
  }
}

export default function LoginPage() {
  const {userProfile, setUserProfile} = useGlobal()
    const [password, setPassword] = useState("")
    const [email, setEmail] = useState("")
    
    const navigate = useNavigate()


      useEffect(() => {
        if (Object.keys(userProfile).length > 1) {
          navigate("/home")
        }
      }, [userProfile])
    

 const sendInputToBack = (_loginMsg) => {
        var req_body = {
            Email: _loginMsg.email,
            Password: _loginMsg.pass,
        }
        const options = {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify(req_body),
        };
        fetch(`http://127.0.0.1:${BACK_END_PORT}/${LOGIN_ROUTE}`, options)
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
          setUserProfile(JSON.parse(data))
        })
        
    }

    const handleSubmit = (e) => {
      e.preventDefault();
        var _loginMsg = new LoginMessage(password, email)
        sendInputToBack(_loginMsg);
    }

    return (
        <div className="flex min-h-full flex-1 flex-col justify-center px-6 py-12 lg:px-8">
        <div className="sm:mx-auto sm:w-full sm:max-w-sm">
          <h2 className="mt-10 text-center text-2xl/9 font-bold tracking-tight text-gray-900">
            Sign in to your account
          </h2>
        </div>

        <div className="mt-10 sm:mx-auto sm:w-full sm:max-w-sm">
          <form onSubmit={handleSubmit} className="space-y-6">

         
          <div>
              <label htmlFor="email" className="block text-sm/6 font-medium text-gray-900">
                Email address
              </label>
              <div className="mt-2">
                <input
                  id="email"
                  name="email"
                  onChange={(e) => setEmail(e.target.value)}
                  type="email"
                  value={email}
                  required
                  autoComplete="email"
                  className="block w-full rounded-md bg-white px-3 py-1.5 text-base text-gray-900 outline outline-1 -outline-offset-1 outline-gray-300 placeholder:text-gray-400 focus:outline focus:outline-2 focus:-outline-offset-2 focus:outline-indigo-600 sm:text-sm/6"
                />
              </div>
            </div>

            <div>
              <div className="flex items-center justify-between">
                <label htmlFor="password" className="block text-sm/6 font-medium text-gray-900">
                  Password
                </label>

              </div>
              <div className="mt-2">
                <input
                  id="password"
                  name="password"
                  type="password"
                  onChange={(e) => setPassword(e.target.value)}
                  value={password}
                  required
                  autoComplete="current-password"
                  className="block w-full rounded-md bg-white px-3 py-1.5 text-base text-gray-900 outline outline-1 -outline-offset-1 outline-gray-300 placeholder:text-gray-400 focus:outline focus:outline-2 focus:-outline-offset-2 focus:outline-indigo-600 sm:text-sm/6"
                />
              </div>
            </div>

            <div>
              <button
                type="submit"
                className="flex w-full justify-center rounded-md bg-indigo-600 px-3 py-1.5 text-sm/6 font-semibold text-white shadow-sm hover:bg-indigo-500 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-indigo-600"
              >
                Sign in
              </button>
            </div>
          </form>

          <p className="mt-10 text-center text-sm/6 text-gray-500">
            Not a member?{' '}
            <a href="/register" className="font-semibold text-indigo-600 hover:text-indigo-500">
              Create an account
            </a>
          </p>
        </div>
      </div>
    );



}