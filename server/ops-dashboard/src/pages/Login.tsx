import { useState } from "react";

export function Login({ onLogin }: { onLogin: () => void }) {
  const [key, setKey] = useState("");
  const [error, setError] = useState("");

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    // Test the key against sidecar
    const res = await fetch(
      (import.meta.env.VITE_SIDECAR_URL ?? "http://localhost:9000") + "/sidecar/containers",
      { headers: { Authorization: `Bearer ${key}` } }
    );
    if (res.status === 401) {
      setError("Invalid admin key");
      return;
    }
    sessionStorage.setItem("adminKey", key);
    onLogin();
  };

  return (
    <div className="login">
      <h1>Ops Dashboard</h1>
      <form onSubmit={handleSubmit}>
        <label>Admin Key</label>
        <input
          type="password"
          value={key}
          onChange={(e) => setKey(e.target.value)}
          placeholder="Enter ADMIN_KEY"
        />
        {error && <p className="error">{error}</p>}
        <button type="submit">Login</button>
      </form>
    </div>
  );
}
