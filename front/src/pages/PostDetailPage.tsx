import { useState, type FormEvent } from "react";
import { Link, Navigate, useNavigate, useParams } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import { useAuth } from "../lib/auth-context";
import { formatLongDate } from "../lib/format";
import {
  getGetApiPostsQueryKey,
  getGetApiPostsIdQueryKey,
  useDeleteApiPostsId,
  useGetApiPostsIdSuspense,
  usePutApiPostsId,
} from "../orval/threadyAPI";

function getErrorMessage(error: unknown) {
  return error instanceof Error ? error.message : "変更を保存できませんでした。";
}

export default function PostDetailPage() {
  const { id } = useParams<{ id: string }>();
  const postId = Number(id);

  if (!id || !Number.isInteger(postId) || postId <= 0) {
    return <Navigate to="/board" replace />;
  }

  return <PostDetailContent postId={postId} />;
}
function PostDetailContent({ postId }: { postId: number }) {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { user } = useAuth();
  const { data: post } = useGetApiPostsIdSuspense(postId);
  const updatePost = usePutApiPostsId();
  const deletePost = useDeleteApiPostsId();
  const [isEditing, setIsEditing] = useState(false);
  const [title, setTitle] = useState(post.title ?? "");
  const [content, setContent] = useState(post.content ?? "");
  const [error, setError] = useState<string | null>(null);
  const isOwner = post.author?.id === user?.id;

  const handleUpdate = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!title.trim() || !content.trim()) return;

    setError(null);
    updatePost.mutate(
      { id: postId, data: { title: title.trim(), content: content.trim() } },
      {
        onSuccess: async () => {
          await queryClient.invalidateQueries({
            queryKey: getGetApiPostsIdQueryKey(postId),
          });
          await queryClient.invalidateQueries({ queryKey: getGetApiPostsQueryKey() });
          setIsEditing(false);
        },
        onError: (mutationError) => setError(getErrorMessage(mutationError)),
      },
    );
  };

  const handleDelete = () => {
    if (!window.confirm("この投稿を削除しますか？")) return;

    setError(null);
    deletePost.mutate(
      { id: postId },
      {
        onSuccess: async () => {
          await queryClient.invalidateQueries({ queryKey: getGetApiPostsQueryKey() });
          navigate("/board");
        },
        onError: (mutationError) => setError(getErrorMessage(mutationError)),
      },
    );
  };

  return (
    <div className="detail-page">
      <Link to="/board" className="back-link">
        <span aria-hidden="true">←</span> ボードへ戻る
      </Link>

      <div className="detail-grid">
        <article className="detail-paper">
          <div className="detail-meta">
            <span>POST / {String(post.id).padStart(4, "0")}</span>
            <span>{post.author?.username ?? "unknown"}</span>
          </div>
          {isEditing ? (
            <form className="edit-form" onSubmit={handleUpdate}>
              <label className="field">
                <span className="field-label">タイトル</span>
                <input
                  className="text-input detail-title-input"
                  value={title}
                  onChange={(event) => setTitle(event.target.value)}
                  maxLength={200}
                  required
                />
              </label>
              <label className="field">
                <span className="field-label">本文</span>
                <textarea
                  className="text-input text-area detail-content-input"
                  value={content}
                  onChange={(event) => setContent(event.target.value)}
                  required
                />
              </label>
              {error && <p className="form-error" role="alert">{error}</p>}
              <div className="detail-actions">
                <button
                  className="button button-secondary"
                  type="button"
                  onClick={() => setIsEditing(false)}
                >
                  キャンセル
                </button>
                <button className="button button-primary" type="submit" disabled={updatePost.isPending}>
                  {updatePost.isPending ? "保存中…" : "変更を保存"} <span aria-hidden="true">↗</span>
                </button>
              </div>
            </form>
          ) : (
            <>
              <h1 className="detail-title">{post.title}</h1>
              <div className="detail-content">{post.content}</div>
              <div className="detail-paper-footer">
                <span>Created {formatLongDate(post.createdAt)}</span>
                {post.updatedAt && post.updatedAt !== post.createdAt && (
                  <span>Updated {formatLongDate(post.updatedAt)}</span>
                )}
              </div>
            </>
          )}
        </article>

        <aside className="detail-aside">
          <div className="detail-aside-block">
            <span className="card-label">Thread note</span>
            <p>投稿は、あとから戻ってこられる小さな入口です。</p>
          </div>
          <div className="fact-list">
            <div className="fact-row">
              <span className="fact-label">AUTHOR</span>
              <strong>{post.author?.username ?? "unknown"}</strong>
            </div>
            <div className="fact-row">
              <span className="fact-label">ACCESS</span>
              <strong>READ ALL · WRITE OWNER</strong>
            </div>
            <div className="fact-row">
              <span className="fact-label">ENDPOINT</span>
              <strong>/api/posts/{postId}</strong>
            </div>
          </div>
          {isOwner && !isEditing && (
            <div className="detail-aside-actions">
              <button
                className="button button-secondary button-wide"
                type="button"
                onClick={() => {
                  setTitle(post.title ?? "");
                  setContent(post.content ?? "");
                  setIsEditing(true);
                }}
              >
                投稿を編集
              </button>
              <button className="button-text danger-link" type="button" onClick={handleDelete} disabled={deletePost.isPending}>
                {deletePost.isPending ? "削除中…" : "この投稿を削除する"}
              </button>
            </div>
          )}
          {error && !isEditing && <p className="form-error" role="alert">{error}</p>}
        </aside>
      </div>
    </div>
  );
}
