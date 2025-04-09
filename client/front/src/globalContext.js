import { createContext, useContext, useState } from 'react';

const GlobalContext = createContext();

export const GlobalProvider = ({ children }) => {
 // const [userProfile, setUserProfile] = useState({UserId:23,Email:"smithjamie@example.net",Firstname:"Taylor",Lastname:"Murphy",Descr:"Student"});
  const [userProfile, setUserProfile] = useState({});
  const [selectedContactId, setSelectedContactId] = useState(-1);
  const [renderedMessages, setRenderedMessages] = useState([]);
  const [allUsers, setAllUsers] = useState([]);

  return (
    <GlobalContext.Provider value={{ userProfile, 
        setUserProfile, 
        selectedContactId, 
        setSelectedContactId, 
        renderedMessages, 
        setRenderedMessages , 
        allUsers, 
        setAllUsers}}>
      {children}
    </GlobalContext.Provider>
  );
};

export const useGlobal = () => {
  const context = useContext(GlobalContext);
  if (!context) {
    throw new Error('useGlobal must be used within a GlobalProvider');
  }
  return context;
};