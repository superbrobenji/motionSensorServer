import logoDark from "../components/welcome/logo-dark.svg";

export default function Index() {
  return (
    <main className="flex items-center justify-center pt-16 pb-4">
      <div className="flex-1 flex flex-col items-center gap-16 min-h-0">
        <header className="flex flex-col items-center gap-9">
          <div className="w-[500px] max-w-[100vw] p-4">
            <img
              src={logoDark}
              alt="React Router"
              className="hidden w-full dark:block"
            />
            <br />
            <h1>Playtopia POC Admin Dashboard</h1>
          </div>
        </header>
      </div>
    </main>
  );
}
