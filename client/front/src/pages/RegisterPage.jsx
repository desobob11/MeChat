
import ChatList from '../components/ChatList';
import Navbar from '../components/Navbar';
import ChatBox from '../components/ChatBox';
import { Link, useNavigate } from 'react-router-dom';
import { useEffect, useState } from 'react';
import { REGISTER_ROUTE, BACK_END_PORT } from '../const';
import { GlobalProvider, useGlobal } from '../globalContext';
/**
 * Register page component
 * 
 * Based on Tailwind templates:
 * https://tailwindcss.com/plus/ui-blocks/application-ui/forms/sign-in-forms
 * 
 * 
 */

// Json object format of 'create account' message that is sent to backend
class CreateAccountMessage {
  constructor(pass, email, first, last, descr) {
    this.email = email;
    this.pass = pass;
    this.first = first;
    this.last = last;
    this.descr = descr;
  }
}



/**
 * Main logic component
 * 
 * @returns 
 * 
 */
export default function RegisterPage() {
  const {userProfile, setUserProfile} = useGlobal()   // sets global user profile on successful login

  // form inputs
  const [password, setPassword] = useState("")
  const [email, setEmail] = useState("")
  const [firstname, setFirstname] = useState("")
  const [lastname, setLastname] = useState("")
  const [descr, setDescr] = useState("")


  const navigate = useNavigate()

  useEffect(() => {
    if (Object.keys(userProfile).length > 1) {    // route to home page if account creation successful
      navigate("/home")
    }
  }, [userProfile])


  // send details to backend, try to create user
  // gets user profile details if created successfully
    const sendInputToBack = (_createMsg) => {
        var req_body = {
            Email: _createMsg.email,
            Password: _createMsg.pass,
            Firstname: _createMsg.first,
            Lastname: _createMsg.last,
            Descr: _createMsg.descr
        }
        const options = {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify(req_body),
        };
        fetch(`http://127.0.0.1:${BACK_END_PORT}/${REGISTER_ROUTE}`, options)
        .then(response => {
          if (!response.ok) {
            alert("Error creating an account. Try different email or please try again later")
            return "{}"
          }
          else {
            alert("User created successfully!")
            return response.text()
          }
        })
        .then(data => {
          setUserProfile(JSON.parse(data))
        })
        
    }

    const handleSubmit = (e) => {
      e.preventDefault();
        var _createMsg = new CreateAccountMessage(password, email, firstname, lastname, descr)
        sendInputToBack(_createMsg);
    }

    return (
        <div className="flex min-h-full flex-1 flex-col justify-center px-6 py-12 lg:px-8">
        <div className="sm:mx-auto sm:w-full sm:max-w-sm">
          <h2 className="mt-10 text-center text-2xl/9 font-bold tracking-tight text-gray-900">
            Create your account
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
              <label htmlFor="firstname" className="block text-sm/6 font-medium text-gray-900">
                First name
              </label>
              <div className="mt-2">
                <input
                  id="firstname"
                  name="firstname"
                  required
                  onChange={(e) => setFirstname(e.target.value)}
                  value={firstname}
                  className="block w-full rounded-md bg-white px-3 py-1.5 text-base text-gray-900 outline outline-1 -outline-offset-1 outline-gray-300 placeholder:text-gray-400 focus:outline focus:outline-2 focus:-outline-offset-2 focus:outline-indigo-600 sm:text-sm/6"
                />
              </div>
            </div>

            <div>
              <label htmlFor="lastname" className="block text-sm/6 font-medium text-gray-900">
                Last name
              </label>
              <div className="mt-2">
                <input
                value={lastname}
                  id="lastname"
                  name="lastname"
                  onChange={(e) => setLastname(e.target.value)}
                  required
                  className="block w-full rounded-md bg-white px-3 py-1.5 text-base text-gray-900 outline outline-1 -outline-offset-1 outline-gray-300 placeholder:text-gray-400 focus:outline focus:outline-2 focus:-outline-offset-2 focus:outline-indigo-600 sm:text-sm/6"
                />
              </div>
            </div>

            <div>
              <label htmlFor="descr" className="block text-sm/6 font-medium text-gray-900">
                Description
              </label>
              <div className="mt-2">
                <input
                value={descr}
                  id="descr"
                  name="descr"
                  onChange={(e) => setDescr(e.target.value)}
                  required
                  className="block w-full rounded-md bg-white px-3 py-1.5 text-base text-gray-900 outline outline-1 -outline-offset-1 outline-gray-300 placeholder:text-gray-400 focus:outline focus:outline-2 focus:-outline-offset-2 focus:outline-indigo-600 sm:text-sm/6"
                />
              </div>
            </div>




            <div>
              <button
                type="submit"
                className="flex w-full justify-center rounded-md bg-indigo-600 px-3 py-1.5 text-sm/6 font-semibold text-white shadow-sm hover:bg-indigo-500 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-indigo-600"
              >
                Create
              </button>
            </div>
          </form>

         
        </div>
      </div>
    );



}