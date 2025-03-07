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

function App() {
  return (
    <div>
    <Helmet>
      <meta name="theme-color" content="#673AAC" />
    </Helmet>
    <Router>

      <Routes>
      <Route path="/home" element={<HomePage/>}/>
        <Route path="/" element={<Navigate to="/home" replace={true} />}/>
      </Routes>
    </Router>
    </div>
  );
}

export default App;
