import { BrowserRouter, Routes, Route, Link } from "react-router-dom";
import { Tracker } from "./pages/Tracker";
import { ApiDocs } from "./pages/ApiDocs";
import { Guides } from "./pages/Guides";
import './App.css'

export default function App() {
  return (
    <BrowserRouter>
      <nav>
        <Link to="/">Live Tracker</Link>
        <Link to="/api">API Reference</Link>
        <Link to="/guides">Integration Guides</Link>
      </nav>
      <Routes>
        <Route path="/" element={<Tracker />} />
        <Route path="/api" element={<ApiDocs />} />
        <Route path="/guides" element={<Guides />} />
      </Routes>
    </BrowserRouter>
  );
}
