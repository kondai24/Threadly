import { useState, type FormEvent } from "react";
import { Link } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import { useAuth } from "../lib/auth-context";
import { formatDate } from "../lib/format";
import {
  getGetApiPostsQueryKey,
  useGetApiPostsSuspense,
  usePostApiPosts,
} from "../orval/threadyAPI";

function getErrorMessage(error: unknown) {
  return error instanceof Error ? error.message : "投稿を保存できませんでした。";
}

export default function BoardPage() {
  const { user } = useAuth();
  const queryClient = useQueryClient();
  const { data } = useGetApiPostsSuspense();
  const createPost = usePostApiPosts();
  const [title, setTitle] = useState("");
  const [content, setContent] = useState("");
  const [formError, setFormError] = useState<string | null>(null);
  const posts = data ?? [];

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const nextTitle = title.trim();
    const nextContent = content.trim();
    if (!nextTitle || !nextContent) return;

    setFormError(null);
    createPost.mutate(
      { data: { title: nextTitle, content: nextContent } },
      {
        onSuccess: () => {
          setTitle("");
          setContent("");
          queryClient.invalidateQueries({ queryKey: getGetApiPostsQueryKey() });
        },
        onError: (error) => setFormError(getErrorMessage(error)),
      },
    );
  };

  return (
    <div>
      <header className="mb-10 flex flex-col gap-6 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <span className="text-xs font-semibold uppercase tracking-[0.16em] text-[#8b8b9e]">
            YOUR BOARD / {user?.username}
          </span>
          <h1 className="mt-3 text-3xl font-extrabold tracking-[-0.03em] sm:text-4xl">考えの流れ</h1>
          <p className="mt-3 text-sm leading-[1.7] text-[#8b8b9e]">
            ここに置いた言葉は、あなたの次の思考への目印になります。
          </p>
        </div>
        <div className="flex items-end gap-3 text-[#8b8b9e]" aria-label="投稿数">
          <strong className="text-4xl font-black leading-none text-[#6c63ff]">
            {posts.length.toString().padStart(2, "0")}
          </strong>
          <span className="text-xs leading-[1.4]">posts<br />in your thread</span>
        </div>
      </header>

      <div className="grid gap-6 lg:grid-cols-[minmax(0,0.85fr)_minmax(0,1.15fr)]">
        <section className="rounded-2xl border border-white/[0.08] bg-white/[0.04] p-6 sm:p-8" aria-labelledby="composer-title">
          <div className="mb-6 flex items-start justify-between">
            <div>
              <span className="text-[0.65rem] font-semibold uppercase tracking-[0.16em] text-[#6c63ff]">New entry</span>
              <h2 id="composer-title" className="mt-2 text-xl font-bold">今日の考えを置く</h2>
            </div>
            <span className="text-3xl font-light text-[#6c63ff]" aria-hidden="true">+</span>
          </div>
          <form className="space-y-5" onSubmit={handleSubmit}>
            <label className="block">
              <span className="mb-2 block text-xs font-semibold uppercase tracking-[0.08em] text-[#8b8b9e]">タイトル</span>
              <input
                className="w-full rounded-xl border border-white/[0.08] bg-white/[0.04] px-4 py-3 text-sm text-[#f0f0f5] outline-none transition-all placeholder:text-[#5a5a6e] focus:border-[#6c63ff] focus:ring-[3px] focus:ring-[#6c63ff]/25"
                type="text"
                value={title}
                onChange={(event) => setTitle(event.target.value)}
                placeholder="考えの見出しをつける"
                maxLength={200}
                required
              />
            </label>
            <label className="block">
              <span className="mb-2 block text-xs font-semibold uppercase tracking-[0.08em] text-[#8b8b9e]">本文</span>
              <textarea
                className="min-h-40 w-full resize-y rounded-xl border border-white/[0.08] bg-white/[0.04] px-4 py-3 text-sm leading-[1.7] text-[#f0f0f5] outline-none transition-all placeholder:text-[#5a5a6e] focus:border-[#6c63ff] focus:ring-[3px] focus:ring-[#6c63ff]/25"
                value={content}
                onChange={(event) => setContent(event.target.value)}
                placeholder="まだまとまっていなくても大丈夫です。"
                required
              />
            </label>
            {formError && <p className="text-sm text-[#f87171]" role="alert">{formError}</p>}
            <div className="flex flex-col gap-4 border-t border-white/[0.08] pt-5 sm:flex-row sm:items-center sm:justify-between">
              <span className="text-xs text-[#5a5a6e]">POST /api/posts · owner only</span>
              <button
                className="inline-flex items-center justify-center gap-2 rounded-xl bg-gradient-to-br from-[#6c63ff] to-[#8b7bff] px-6 py-3 text-sm font-semibold text-white shadow-[0_4px_20px_rgba(108,99,255,0.25)] outline-none transition-all hover:-translate-y-0.5 focus-visible:ring-2 focus-visible:ring-[#6c63ff]/50 disabled:cursor-not-allowed disabled:opacity-50"
                type="submit"
                disabled={createPost.isPending || !title.trim() || !content.trim()}
              >
                {createPost.isPending ? "保存中…" : "投稿を残す"} ↗
              </button>
            </div>
          </form>
        </section>

        <section aria-labelledby="feed-title">
          <div className="mb-5 flex items-end justify-between gap-4 border-b border-white/[0.08] pb-4">
            <div>
              <span className="text-[0.65rem] font-semibold uppercase tracking-[0.16em] text-[#6c63ff]">Archive / {posts.length}</span>
              <h2 id="feed-title" className="mt-2 text-xl font-bold">Recent thoughts</h2>
            </div>
            <span className="text-right text-xs text-[#5a5a6e]">read: all · write: {user?.username}</span>
          </div>

          {posts.length === 0 ? (
            <div className="rounded-2xl border border-dashed border-white/[0.12] px-6 py-16 text-center">
              <span className="text-4xl font-black text-[#6c63ff]/40">01</span>
              <h3 className="mt-4 text-lg font-bold">最初の一枚を置いてみる</h3>
              <p className="mx-auto mt-2 max-w-sm text-sm leading-[1.7] text-[#8b8b9e]">左のフォームからタイトルと本文を書けば、ここに流れが生まれます。</p>
            </div>
          ) : (
            <div className="space-y-3">
              {posts.map((post, index) => (
                <Link
                  to={`/board/${post.id}`}
                  className="group flex items-start gap-4 rounded-xl border border-white/[0.08] bg-white/[0.04] p-4 no-underline outline-none transition-all hover:translate-x-1 hover:border-white/[0.15] hover:bg-white/[0.07] focus-visible:ring-2 focus-visible:ring-[#6c63ff]/50"
                  key={post.id}
                >
                  <span className="pt-1 text-xs font-bold text-[#6c63ff]">{String(index + 1).padStart(2, "0")}</span>
                  <span className="min-w-0 flex-1">
                    <span className="flex flex-wrap gap-x-3 gap-y-1 text-xs text-[#5a5a6e]">
                      <span>{post.author?.username ?? "unknown"}</span>
                      <span>{formatDate(post.createdAt)}</span>
                    </span>
                    <strong className="mt-2 block truncate text-base text-[#f0f0f5]">{post.title}</strong>
                    <span className="mt-3 flex items-center justify-between text-[0.65rem] font-semibold uppercase tracking-[0.12em] text-[#5a5a6e]">
                      <span>OPEN THREAD</span>
                      <span className="text-sm transition-transform group-hover:translate-x-1">↗</span>
                    </span>
                  </span>
                </Link>
              ))}
            </div>
          )}
        </section>
      </div>
    </div>
  );
}
