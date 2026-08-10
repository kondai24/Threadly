import { useState, type FormEvent } from "react";
import { Link, Navigate, useNavigate } from "react-router-dom";
import {
  usePostApiAuthLogin,
  usePostApiAuthRegister,
} from "../orval/threadyAPI";
import { useAuth } from "../lib/auth-context";

type AuthMode = "login" | "register";

export default function AuthPage({ mode }: { mode: AuthMode }) {
  const navigate = useNavigate();
  const { isAuthenticated, setSession } = useAuth();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const login = usePostApiAuthLogin();
  const register = usePostApiAuthRegister();
  const mutation = mode === "login" ? login : register;
  const isRegister = mode === "register";

  if (isAuthenticated) return <Navigate to="/board" replace />;

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setError(null);

    mutation.mutate(
      { data: { username: username.trim(), password } },
      {
        onSuccess: (response) => {
          setSession(response);
          navigate("/board");
        },
        onError: (mutationError) => {
          setError(
            mutationError instanceof Error
              ? mutationError.message
              : isRegister
                ? "アカウントを作成できませんでした。"
                : "ログインできませんでした。",
          );
        },
      },
    );
  };

  return (
    <div className="auth-page">
      <section className="auth-copy">
        <span className="auth-kicker">
          THREADLY / SESSION {isRegister ? "02" : "01"}
        </span>
        <h1 className="auth-title">
          {isRegister ? "会話の席を、ひとつ。" : "続きを、ここから。"}
        </h1>
        <p>
          {isRegister
            ? "あなたの名前で投稿を残し、あとから何度でも会話に戻れます。"
            : "あなたのボードへ戻って、考えの続きを書きましょう。"}
        </p>
        <div className="auth-note">
          <span className="signal-dot" aria-hidden="true" />
          <span>JWTセッションで安全に接続</span>
        </div>
      </section>

      <section className="auth-form-panel">
        <div className="auth-panel-heading">
          <span className="eyebrow">{isRegister ? "New account" : "Welcome back"}</span>
          <h2>{isRegister ? "アカウントを作成" : "ログイン"}</h2>
        </div>
        <form className="auth-form" onSubmit={handleSubmit}>
          <label className="field">
            <span className="field-label">ユーザー名</span>
            <input
              className="text-input"
              type="text"
              value={username}
              onChange={(event) => setUsername(event.target.value)}
              placeholder="たとえば、daisuke"
              autoComplete="username"
              minLength={3}
              maxLength={32}
              required
            />
            <span className="field-help">英数字とアンダースコア、3〜32文字</span>
          </label>
          <label className="field">
            <span className="field-label">パスワード</span>
            <input
              className="text-input"
              type="password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              placeholder="8文字以上"
              autoComplete={isRegister ? "new-password" : "current-password"}
              minLength={8}
              maxLength={128}
              required
            />
          </label>
          {error && (
            <p className="form-error" role="alert">
              {error}
            </p>
          )}
          <button
            className="button button-primary button-wide"
            type="submit"
            disabled={mutation.isPending}
          >
            {mutation.isPending
              ? "接続しています…"
              : isRegister
                ? "アカウントを作成"
                : "ログインする"}
            <span aria-hidden="true">↗</span>
          </button>
        </form>
        <div className="auth-switch">
          <span>{isRegister ? "すでにアカウントがありますか？" : "はじめてですか？"}</span>
          <Link to={isRegister ? "/login" : "/register"}>
            {isRegister ? "ログイン" : "アカウントを作成"}
          </Link>
        </div>
      </section>
    </div>
  );
}
