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
    <div className="grid min-h-[calc(100vh-8rem)] items-center gap-10 lg:grid-cols-[1fr_0.8fr]">
      <section className="hidden lg:block">
        <span className="text-xs font-semibold uppercase tracking-[0.16em] text-[#8b8b9e]">
          THREADLY / SESSION {isRegister ? "02" : "01"}
        </span>
        <h1 className="mt-6 max-w-lg text-5xl font-black leading-[1.08] tracking-[-0.04em]">
          {isRegister ? "会話の席を、ひとつ。" : "続きを、ここから。"}
        </h1>
        <p className="mt-6 max-w-md text-base leading-[1.8] text-[#8b8b9e]">
          {isRegister
            ? "あなたの名前で投稿を残し、あとから何度でも会話に戻れます。"
            : "あなたのボードへ戻って、考えの続きを書きましょう。"}
        </p>
        <div className="mt-8 flex items-center gap-2 text-xs text-[#8b8b9e]">
          <span className="h-2 w-2 animate-pulse rounded-full bg-[#4ade80]" aria-hidden="true" />
          JWTセッションで安全に接続
        </div>
      </section>

      <section className="rounded-3xl border border-white/[0.08] bg-white/[0.04] p-6 sm:p-10">
        <div className="mb-8">
          <span className="text-[0.65rem] font-semibold uppercase tracking-[0.16em] text-[#6c63ff]">
            {isRegister ? "New account" : "Welcome back"}
          </span>
          <h2 className="mt-3 text-2xl font-bold">{isRegister ? "アカウントを作成" : "ログイン"}</h2>
        </div>
        <form className="space-y-5" onSubmit={handleSubmit}>
          <label className="block">
            <span className="mb-2 block text-xs font-semibold uppercase tracking-[0.08em] text-[#8b8b9e]">ユーザー名</span>
            <input
              className="w-full rounded-xl border border-white/[0.08] bg-white/[0.04] px-4 py-3 text-sm text-[#f0f0f5] outline-none transition-all placeholder:text-[#5a5a6e] focus:border-[#6c63ff] focus:ring-[3px] focus:ring-[#6c63ff]/25"
              type="text"
              value={username}
              onChange={(event) => setUsername(event.target.value)}
              placeholder="ユーザー名を入力してください"
              autoComplete="username"
              minLength={3}
              maxLength={32}
              required
            />
            <span className="mt-2 block text-xs text-[#5a5a6e]">英数字とアンダースコア、3〜32文字</span>
          </label>
          <label className="block">
            <span className="mb-2 block text-xs font-semibold uppercase tracking-[0.08em] text-[#8b8b9e]">パスワード</span>
            <input
              className="w-full rounded-xl border border-white/[0.08] bg-white/[0.04] px-4 py-3 text-sm text-[#f0f0f5] outline-none transition-all placeholder:text-[#5a5a6e] focus:border-[#6c63ff] focus:ring-[3px] focus:ring-[#6c63ff]/25"
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
          {error && <p className="text-sm text-[#f87171]" role="alert">{error}</p>}
          <button
            className="inline-flex w-full items-center justify-center gap-2 rounded-xl bg-gradient-to-br from-[#6c63ff] to-[#8b7bff] px-6 py-3 text-sm font-semibold text-white shadow-[0_4px_20px_rgba(108,99,255,0.25)] transition-all hover:-translate-y-0.5 disabled:cursor-not-allowed disabled:opacity-50"
            type="submit"
            disabled={mutation.isPending}
          >
            {mutation.isPending ? "接続しています…" : isRegister ? "アカウントを作成" : "ログインする"} ↗
          </button>
        </form>
        <div className="mt-8 flex flex-wrap gap-2 border-t border-white/[0.08] pt-5 text-xs text-[#8b8b9e]">
          <span>{isRegister ? "すでにアカウントがありますか？" : "はじめてですか？"}</span>
          <Link className="text-[#6c63ff] no-underline hover:text-[#8b7bff]" to={isRegister ? "/login" : "/register"}>
            {isRegister ? "ログイン" : "アカウントを作成"}
          </Link>
        </div>
      </section>
    </div>
  );
}
