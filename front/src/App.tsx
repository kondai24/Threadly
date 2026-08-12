import { Suspense, type ReactNode } from "react";
import { ErrorBoundary } from "react-error-boundary";
import { Link, Navigate, Route, Routes } from "react-router-dom";
import Layout from "./components/Layout";
import { useAuth } from "./lib/auth-context";
import AuthPage from "./pages/AuthPage";
import BoardPage from "./pages/BoardPage";
import HomePage from "./pages/HomePage";
import PostDetailPage from "./pages/PostDetailPage";

function LoadingFallback() {
  return (
    <div className="flex flex-col items-center justify-center py-16 text-center">
      <div className="mb-4 h-9 w-9 animate-spin rounded-full border-[3px] border-white/[0.08] border-t-[#6c63ff]" />
      <p className="text-sm text-[#8b8b9e]">読み込み中...</p>
    </div>
  );
}

function ErrorFallback({
  error,
  resetErrorBoundary,
}: {
  error: unknown;
  resetErrorBoundary: () => void;
}) {
  const is404 =
    error instanceof Error &&
    (error as Error & { status?: number }).status === 404;

  return (
    <div className="flex flex-col items-center justify-center py-16 text-center">
      <span className="mb-3 text-xs font-semibold uppercase tracking-[0.16em] text-[#8b8b9e]">
        接続エラー
      </span>
      <h2 className="text-2xl font-extrabold tracking-[-0.02em] text-[#f0f0f5]">
        {is404 ? "投稿が見つかりませんでした" : "読み込みに失敗しました"}
      </h2>
      <p className="mt-3 max-w-md text-sm leading-[1.7] text-[#8b8b9e]">
        {is404
          ? "リンク先の投稿は削除されたか、あなたからは見えません。"
          : error instanceof Error
            ? error.message
            : "時間を置いてもう一度試してください。"}
      </p>
      <Link
        to="/board"
        className="mt-6 inline-flex items-center justify-center gap-2 rounded-xl border border-white/[0.08] bg-white/[0.06] px-6 py-3 text-[0.9rem] font-semibold text-[#f0f0f5] no-underline outline-none transition-all duration-200 hover:border-white/[0.15] hover:bg-white/[0.07] focus-visible:ring-2 focus-visible:ring-[#6c63ff]/50"
        onClick={() => resetErrorBoundary()}
      >
        ボードへ戻る
      </Link>
    </div>
  );
}

function ProtectedRoute({ children }: { children: ReactNode }) {
  const { isAuthenticated, isLoading } = useAuth();

  if (isLoading) return <LoadingFallback />;
  if (!isAuthenticated) return <Navigate to="/login" replace />;
  return children;
}

function ProtectedPage({ children }: { children: ReactNode }) {
  return (
    <ProtectedRoute>
      <ErrorBoundary FallbackComponent={ErrorFallback}>
        <Suspense fallback={<LoadingFallback />}>{children}</Suspense>
      </ErrorBoundary>
    </ProtectedRoute>
  );
}

export default function App() {
  return (
    <Routes>
      <Route element={<Layout />}>
        <Route path="/" element={<HomePage />} />
        <Route path="/login" element={<AuthPage mode="login" />} />
        <Route path="/register" element={<AuthPage mode="register" />} />
        <Route
          path="/board"
          element={
            <ProtectedPage>
              <BoardPage />
            </ProtectedPage>
          }
        />
        <Route
          path="/board/:id"
          element={
            <ProtectedPage>
              <PostDetailPage />
            </ProtectedPage>
          }
        />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Route>
    </Routes>
  );
}
