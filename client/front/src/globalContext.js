import { createContext, useContext, useState } from 'react';

const GlobalContext = createContext();

export const GlobalProvider = ({ children }) => {
  const [userProfile, setUserProfile] = useState({UserId:1,Email:"admin@admin.ca",Firstname:"admin",Lastname:"admin",Descr:"Student"});
  const [selectedContactId, setSelectedContactId] = useState(-1);
  const [renderedMessages, setRenderedMessages] = useState([]);

  return (
    <GlobalContext.Provider value={{ userProfile, setUserProfile, selectedContactId, setSelectedContactId }}>
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