import { Link, Outlet, useLocation } from "react-router-dom";

export default function Layout() {
  const location = useLocation();

  return (
    <>
      <nav className="navbar">
        <Link to="/" className="navbar-brand">
          <span className="logo-icon">🧵</span>
          Thready
        </Link>
        <div className="navbar-links">
          <Link to="/" className={location.pathname === "/" ? "active" : ""}>
            ホーム
          </Link>
          <Link
            to="/board"
            className={location.pathname.startsWith("/board") ? "active" : ""}
          >
            掲示板
          </Link>
        </div>
      </nav>
      <main className="main-content">
        <Outlet />
      </main>
    </>
  );
}
