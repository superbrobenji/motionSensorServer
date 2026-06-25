import { NavLink } from "react-router";

export function Navigation() {
  const pages = [
    { pageName: "Home", route: "/" },
    { pageName: "Nodes", route: "/nodes" },
    { pageName: "Enrollments", route: "/enrollments" },
    { pageName: "Server", route: "/server" },
  ];
  return (
    <nav className="bg-gray-800 border-0 sm:rounded-r-md flex sm:flex-col justify-center gap-4 py-4 sm:py-0 px-6 sm:h-[100vh]">
      {pages.map((page, index) => (
        <NavLink
          key={`${page.pageName}-${index}`}
          to={page.route}
          className={({ isActive, isPending, isTransitioning }) =>
            [
              isPending ? "pending" : "",
              isActive ? "active" : "",
              isTransitioning ? "transitioning" : "",
            ].join(" ")
          }
          end>
          {page.pageName}
        </NavLink>
      ))}
    </nav>
  );
}
