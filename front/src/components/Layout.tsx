import { Link, NavLink, Outlet, useNavigate } from "react-router-dom";
import { useAuth } from "../lib/auth-context";

const navLinkClass = ({ isActive }: { isActive: boolean }) =>
  `flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium no-underline transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#6c63ff]/50 ${
    isActive
      ? "bg-white/[0.07] text-[#f0f0f5]"
      : "text-[#8b8b9e] hover:bg-white/[0.07] hover:text-[#f0f0f5]"
  }`;

export default function Layout() {
  const navigate = useNavigate();
  const { isAuthenticated, signOut, user } = useAuth();

  const handleSignOut = () => {
    signOut();
    navigate("/");
  };

  return (
    <div className="min-h-screen bg-[#0a0a0f] font-sans leading-[1.6] text-[#f0f0f5] antialiased">
      <aside className="fixed inset-y-0 left-0 z-40 hidden w-64 flex-col justify-between border-r border-white/[0.08] bg-[#0a0a0f] px-6 py-8 md:flex">
        <div>
          <Link
            to="/"
            className="flex items-center gap-3 text-xl font-extrabold tracking-[-0.02em] text-[#f0f0f5] no-underline"
            aria-label="Threadly home"
          >
            <span className="flex h-9 w-9 items-center justify-center rounded-lg bg-gradient-to-br from-[#6c63ff] to-[#ff6b9d] text-base">
              🧵
            </span>
            <span>
              <span className="block">Threadly</span>
              <span className="block text-[0.62rem] font-medium uppercase tracking-[0.16em] text-[#5a5a6e]">
                conversation studio
              </span>
            </span>
          </Link>

          <nav className="mt-12 space-y-2" aria-label="メインナビゲーション">
            <span className="mb-3 block px-3 text-[0.65rem] font-semibold uppercase tracking-[0.16em] text-[#5a5a6e]">
              Navigate
            </span>
            <NavLink to="/" end className={navLinkClass}>
              <span className="w-6 text-xs text-[#6c63ff]">00</span>
              <span>ホーム</span>
            </NavLink>
            <NavLink to="/board" className={navLinkClass}>
              <span className="w-6 text-xs text-[#6c63ff]">01</span>
              <span>マイボード</span>
            </NavLink>
          </nav>
        </div>

        <div className="space-y-4 text-sm text-[#8b8b9e]">
          {isAuthenticated && user ? (
            <>
              <div className="flex items-center gap-3 rounded-xl border border-white/[0.08] bg-white/[0.04] p-3">
                <span className="flex h-8 w-8 items-center justify-center rounded-full bg-[#6c63ff] text-sm font-bold text-white">
                  {user.username?.slice(0, 1).toUpperCase()}
                </span>
                <span className="min-w-0">
                  <span className="block text-[0.65rem] uppercase tracking-[0.12em] text-[#5a5a6e]">
                    Signed in as
                  </span>
                  <strong className="block truncate text-[#f0f0f5]">{user.username}</strong>
                </span>
              </div>
              <button
                className="w-full text-left text-xs text-[#8b8b9e] transition-colors hover:text-[#f0f0f5]"
                type="button"
                onClick={handleSignOut}
              >
                セッションを終了 ↗
              </button>
            </>
          ) : (
            <div className="space-y-3 rounded-xl border border-white/[0.08] bg-white/[0.04] p-4">
              <span className="block text-[0.65rem] font-semibold uppercase tracking-[0.16em] text-[#5a5a6e]">
                Your desk
              </span>
              <p className="text-xs leading-[1.6]">ログインすると投稿を残せます。</p>
              <Link
                to="/login"
                className="inline-flex items-center justify-center gap-2 rounded-lg bg-gradient-to-br from-[#6c63ff] to-[#8b7bff] px-4 py-2 text-xs font-semibold text-white no-underline shadow-[0_4px_20px_rgba(108,99,255,0.25)] transition-transform hover:-translate-y-0.5"
              >
                ログイン ↗
              </Link>
            </div>
          )}
          <span className="block text-[0.65rem] text-[#5a5a6e]">API / v1.0 · JWT secured</span>
        </div>
      </aside>

      <main className="min-h-screen md:ml-64">
        <header className="sticky top-0 z-30 flex h-16 items-center justify-between border-b border-white/[0.08] bg-[#0a0a0f]/80 px-4 backdrop-blur-[20px] md:hidden">
          <Link to="/" className="text-lg font-extrabold text-[#f0f0f5] no-underline">
            Threadly
          </Link>
          {isAuthenticated ? (
            <button className="text-xs text-[#8b8b9e]" type="button" onClick={handleSignOut}>
              終了
            </button>
          ) : (
            <Link to="/login" className="text-xs text-[#8b8b9e] no-underline">
              ログイン ↗
            </Link>
          )}
        </header>
        <div className="mx-auto w-full max-w-[1100px] px-4 py-6 sm:px-8 sm:py-10">
          <Outlet />
        </div>
      </main>
    </div>
  );
}
