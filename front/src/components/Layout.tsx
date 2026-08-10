import { Link, NavLink, Outlet, useLocation, useNavigate } from "react-router-dom";
import { useAuth } from "../lib/auth-context";

export default function Layout() {
  const location = useLocation();
  const navigate = useNavigate();
  const { isAuthenticated, signOut, user } = useAuth();
  const isAuthPage = location.pathname === "/login" || location.pathname === "/register";

  const handleSignOut = () => {
    signOut();
    navigate("/");
  };

  return (
    <div className={`app-shell${isAuthPage ? " app-shell-auth" : ""}`}>
      <aside className="app-sidebar">
        <div>
          <Link to="/" className="brand-lockup" aria-label="Threadly home">
            <span className="brand-mark" aria-hidden="true">
              <span />
              <span />
              <span />
            </span>
            <span>
              <span className="brand-wordmark">Threadly</span>
              <span className="brand-kicker">conversation studio</span>
            </span>
          </Link>

          <nav className="sidebar-nav" aria-label="メインナビゲーション">
            <span className="nav-label">Navigate</span>
            <NavLink to="/" end className="sidebar-link">
              <span className="nav-index">00</span>
              <span>ホーム</span>
            </NavLink>
            <NavLink to="/board" className="sidebar-link">
              <span className="nav-index">01</span>
              <span>マイボード</span>
            </NavLink>
          </nav>
        </div>

        <div className="sidebar-foot">
          {isAuthenticated && user ? (
            <>
              <div className="profile-chip">
                <span className="avatar">{user.username?.slice(0, 1).toUpperCase()}</span>
                <span>
                  <span className="profile-label">Signed in as</span>
                  <strong>{user.username}</strong>
                </span>
              </div>
              <button className="button-text" type="button" onClick={handleSignOut}>
                セッションを終了 <span aria-hidden="true">↗</span>
              </button>
            </>
          ) : (
            <div className="sidebar-auth">
              <span className="nav-label">Your desk</span>
              <p>ログインすると投稿を残せます。</p>
              <Link to="/login" className="button button-primary button-small">
                ログイン <span aria-hidden="true">↗</span>
              </Link>
            </div>
          )}
          <span className="sidebar-version">API / v1.0 · JWT secured</span>
        </div>
      </aside>

      <main className="page-content">
        <div className="mobile-header">
          <Link to="/" className="brand-wordmark">
            Threadly
          </Link>
          {isAuthenticated ? (
            <button className="button-text" type="button" onClick={handleSignOut}>
              終了
            </button>
          ) : (
            <Link to="/login" className="button-text">
              ログイン ↗
            </Link>
          )}
        </div>
        <div className="page-inner">
          <Outlet />
        </div>
      </main>
    </div>
  );
}
