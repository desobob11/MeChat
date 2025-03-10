import logo from './logo.svg';
import './App.css';
import './components/Navbar.jsx'
import HomePage from './pages/Homepage.jsx';
import {
  BrowserRouter as Router,
  Route,
  Routes,
  Navigate
} from "react-router-dom";
import {Helmet} from "react-helmet"
import LoginPage from './pages/LoginPage.jsx';
import RegisterPage from './pages/RegisterPage.jsx';
import { GlobalProvider } from './globalContext.js';


export 

function App() {
  return (
    <GlobalProvider>
    <div>
    <Helmet>
      <meta name="theme-color" content="#673AAC" />
    </Helmet>
    <Router>

      <Routes>
      <Route path="/login" element={<LoginPage/>}/>
      <Route path="/home" element={<HomePage/>}/>
      <Route path="/register" element={<RegisterPage/>}/>
        <Route path="/" element={<Navigate to="/login" replace={true} />}/>
      </Routes>
    </Router>
    </div>
    </GlobalProvider>
  );
}

export default App;
