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
    <div className="board-page">
      <header className="board-topline">
        <div>
          <span className="eyebrow">YOUR BOARD / {user?.username}</span>
          <h1 className="page-title">考えの流れ</h1>
          <p className="page-lede">
            ここに置いた言葉は、あなたの次の思考への目印になります。
          </p>
        </div>
        <div className="board-summary" aria-label="投稿数">
          <strong>{posts.length.toString().padStart(2, "0")}</strong>
          <span>posts<br />in your thread</span>
        </div>
      </header>

      <div className="board-layout">
        <section className="composer-card" aria-labelledby="composer-title">
          <div className="composer-header">
            <div>
              <span className="card-label">New entry</span>
              <h2 id="composer-title">今日の考えを置く</h2>
            </div>
            <span className="composer-mark" aria-hidden="true">+</span>
          </div>
          <form className="composer-form" onSubmit={handleSubmit}>
            <label className="field">
              <span className="field-label">タイトル</span>
              <input
                className="text-input"
                type="text"
                value={title}
                onChange={(event) => setTitle(event.target.value)}
                placeholder="考えの見出しをつける"
                maxLength={200}
                required
              />
            </label>
            <label className="field">
              <span className="field-label">本文</span>
              <textarea
                className="text-input text-area"
                value={content}
                onChange={(event) => setContent(event.target.value)}
                placeholder="まだまとまっていなくても大丈夫です。"
                required
              />
            </label>
            {formError && (
              <p className="form-error" role="alert">
                {formError}
              </p>
            )}
            <div className="form-footer">
              <span className="field-help">POST /api/posts · owner only</span>
              <button
                className="button button-primary"
                type="submit"
                disabled={createPost.isPending || !title.trim() || !content.trim()}
              >
                {createPost.isPending ? "保存中…" : "投稿を残す"} <span aria-hidden="true">↗</span>
              </button>
            </div>
          </form>
        </section>

        <section className="post-feed" aria-labelledby="feed-title">
          <div className="feed-heading">
            <div>
              <span className="card-label">Archive / {posts.length}</span>
              <h2 id="feed-title">Recent thoughts</h2>
            </div>
            <span className="feed-filter">read: all · write: {user?.username}</span>
          </div>

          {posts.length === 0 ? (
            <div className="empty-state">
              <span className="empty-index">01</span>
              <h3>最初の一枚を置いてみる</h3>
              <p>左のフォームからタイトルと本文を書けば、ここに流れが生まれます。</p>
            </div>
          ) : (
            <div className="post-list">
              {posts.map((post, index) => (
                <Link to={`/board/${post.id}`} className="post-card" key={post.id}>
                  <span className="post-number">{String(index + 1).padStart(2, "0")}</span>
                  <span className="post-card-main">
                    <span className="post-card-meta">
                      <span>{post.author?.username ?? "unknown"}</span>
                      <span>{formatDate(post.createdAt)}</span>
                    </span>
                    <strong className="post-card-title">{post.title}</strong>
                    <span className="post-card-footer">
                      <span>OPEN THREAD</span>
                      <span className="post-arrow" aria-hidden="true">↗</span>
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
