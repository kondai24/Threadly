import { useState } from "react";
import { Link } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import {
  useGetApiPostsSuspense,
  usePostApiPosts,
  getGetApiPostsQueryKey,
} from "../orval/threadyAPI";

export default function BoardPage() {
  const [isDialogOpen, setIsDialogOpen] = useState(false);
  const [title, setTitle] = useState("");
  const [content, setContent] = useState("");

  const queryClient = useQueryClient();
  const { data: posts } = useGetApiPostsSuspense();
  const createPost = usePostApiPosts();

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!title.trim() || !content.trim()) return;

    createPost.mutate(
      { data: { title: title.trim(), content: content.trim() } },
      {
        onSuccess: () => {
          setTitle("");
          setContent("");
          setIsDialogOpen(false);
          queryClient.invalidateQueries({
            queryKey: getGetApiPostsQueryKey(),
          });
        },
      },
    );
  };

  const formatDate = (dateStr?: string) => {
    if (!dateStr) return "";
    try {
      return new Date(dateStr).toLocaleDateString("ja-JP", {
        year: "numeric",
        month: "short",
        day: "numeric",
        hour: "2-digit",
        minute: "2-digit",
      });
    } catch {
      return dateStr;
    }
  };

  return (
    <div>
      <div className="board-header">
        <div>
          <h1>📋 掲示板</h1>
          <p className="post-count">
            {posts.length > 0 ? `${posts.length} 件の投稿` : ""}
          </p>
        </div>
        <button
          className="btn btn-primary"
          onClick={() => setIsDialogOpen(true)}
        >
          ✏️ 新規投稿
        </button>
      </div>

      {posts.length === 0 && (
        <div className="empty-state">
          <div className="empty-icon">📭</div>
          <p>まだ投稿がありません</p>
          <p style={{ color: "var(--text-muted)", marginTop: "0.5rem" }}>
            最初の投稿をしてみましょう！
          </p>
        </div>
      )}

      {posts.length > 0 && (
        <div className="post-list">
          {posts.map((post) => (
            <Link to={`/board/${post.id}`} className="post-item" key={post.id}>
              <div className="post-id">#{post.id}</div>
              <div className="post-info">
                <div className="post-title">{post.title}</div>
                <div className="post-date">{formatDate(post.createdAt)}</div>
              </div>
              <span className="arrow">→</span>
            </Link>
          ))}
        </div>
      )}

      {isDialogOpen && (
        <div
          className="dialog-overlay"
          onClick={(e) => {
            if (e.target === e.currentTarget) setIsDialogOpen(false);
          }}
        >
          <div className="dialog">
            <h2>✏️ 新しい投稿</h2>
            <form onSubmit={handleSubmit}>
              <div className="form-group">
                <label htmlFor="post-title">タイトル</label>
                <input
                  id="post-title"
                  type="text"
                  placeholder="投稿のタイトルを入力..."
                  value={title}
                  onChange={(e) => setTitle(e.target.value)}
                  autoFocus
                />
              </div>
              <div className="form-group">
                <label htmlFor="post-content">内容</label>
                <textarea
                  id="post-content"
                  placeholder="投稿の内容を入力..."
                  value={content}
                  onChange={(e) => setContent(e.target.value)}
                />
              </div>
              <div className="dialog-actions">
                <button
                  type="button"
                  className="btn btn-secondary"
                  onClick={() => setIsDialogOpen(false)}
                >
                  キャンセル
                </button>
                <button
                  type="submit"
                  className="btn btn-primary"
                  disabled={
                    createPost.isPending || !title.trim() || !content.trim()
                  }
                >
                  {createPost.isPending ? "投稿中..." : "投稿する"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
