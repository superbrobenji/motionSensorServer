import { useState } from "react";
import { BrowserRouter, Routes, Route, Link } from "react-router-dom";
import { Login } from "./pages/Login";
import { SystemOverview } from "./pages/SystemOverview";
import { KafkaEvents } from "./pages/KafkaEvents";

export default function App() {
  const [authed, setAuthed] = useState(() => !!sessionStorage.getItem("adminKey"));

  if (!authed) return <Login onLogin={() => setAuthed(true)} />;

  return (
    <BrowserRouter>
      <nav>
        <Link to="/">System Overview</Link>
        <Link to="/kafka">Kafka Events</Link>
        <button onClick={() => { sessionStorage.removeItem("adminKey"); setAuthed(false); }}>
          Logout
        </button>
      </nav>
      <Routes>
        <Route path="/" element={<SystemOverview />} />
        <Route path="/kafka" element={<KafkaEvents />} />
      </Routes>
    </BrowserRouter>
  );
}
