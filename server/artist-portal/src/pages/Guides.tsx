export function Guides() {
  return (
    <div className="guides">
      <h1>Integration Guide</h1>

      <section>
        <h2>Quick Start</h2>
        <p>The Planetopia API is available at your deployment URL on port 8080. No authentication required.</p>
        <pre>{`curl http://localhost:8080/api/v1/nodes`}</pre>
      </section>

      <section>
        <h2>Real-time Events</h2>
        <p>Subscribe to Server-Sent Events for live node data:</p>
        <pre>{`const es = new EventSource("http://localhost:8080/api/v1/events");
es.addEventListener("motion", (e) => console.log(JSON.parse(e.data)));`}</pre>
      </section>

      <section>
        <h2>Sending Commands</h2>
        <p>Send commands to output nodes (LED strips, relays):</p>
        <pre>{`// Light up a node in red
fetch("http://localhost:8080/api/v1/nodes/3/command", {
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify({ action: "led_solid", colour: [255, 0, 0] })
})`}</pre>
      </section>

      <section>
        <h2>Node Types</h2>
        <ul>
          <li><strong>pir</strong> — motion sensor (input — sends events to server)</li>
          <li><strong>led</strong> — LED strip (output — receives commands from server)</li>
          <li><strong>relay</strong> — relay switch (output)</li>
        </ul>
      </section>
    </div>
  );
}
