import { Routes, Route, Link } from "react-router-dom";
import { Suspense } from "react";
import { ErrorBoundary } from "react-error-boundary";
import Layout from "./components/Layout";
import HomePage from "./pages/HomePage";
import BoardPage from "./pages/BoardPage";
import PostDetailPage from "./pages/PostDetailPage";

function LoadingFallback() {
  return (
    <div className="loading-state">
      <div className="spinner"></div>
      <p>読み込み中...</p>
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
  const is404 = error instanceof Error && error.message?.includes("404");
  return (
    <div className="error-state">
      <p>{is404 ? "投稿が見つかりませんでした。" : "エラーが発生しました。"}</p>
      <Link
        to="/board"
        className="btn btn-secondary"
        style={{ marginTop: "1rem" }}
        onClick={() => resetErrorBoundary()}
      >
        ← 掲示板に戻る
      </Link>
    </div>
  );
}

export default function App() {
  return (
    <Routes>
      <Route element={<Layout />}>
        <Route path="/" element={<HomePage />} />
        <Route
          path="/board"
          element={
            <ErrorBoundary FallbackComponent={ErrorFallback}>
              <Suspense fallback={<LoadingFallback />}>
                <BoardPage />
              </Suspense>
            </ErrorBoundary>
          }
        />
        <Route
          path="/board/:id"
          element={
            <ErrorBoundary FallbackComponent={ErrorFallback}>
              <Suspense fallback={<LoadingFallback />}>
                <PostDetailPage />
              </Suspense>
            </ErrorBoundary>
          }
        />
      </Route>
    </Routes>
  );
}
