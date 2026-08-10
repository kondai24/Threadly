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
    <div className="state-card loading-state">
      <div className="spinner" aria-hidden="true" />
      <p>会話を読み込んでいます…</p>
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
    <div className="state-card error-state">
      <span className="eyebrow">接続エラー</span>
      <h2>{is404 ? "投稿が見つかりませんでした" : "読み込みに失敗しました"}</h2>
      <p>
        {is404
          ? "リンク先の投稿は削除されたか、あなたからは見えません。"
          : error instanceof Error
            ? error.message
            : "時間を置いてもう一度試してください。"}
      </p>
      <Link
        to="/board"
        className="button button-secondary"
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
