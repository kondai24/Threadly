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
  const postId = id ?? "";
  const isUUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i.test(postId);

  if (!isUUID) {
    return <Navigate to="/board" replace />;
  }

  return <PostDetailContent postId={postId} />;
}

function PostDetailContent({ postId }: { postId: string }) {
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
          await queryClient.invalidateQueries({ queryKey: getGetApiPostsIdQueryKey(postId) });
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
    <div>
      <Link
        to="/board"
        className="mb-8 inline-flex items-center gap-2 text-sm font-medium text-[#8b8b9e] no-underline outline-none transition-colors hover:text-[#6c63ff] focus-visible:ring-2 focus-visible:ring-[#6c63ff]/50"
      >
        ← ボードへ戻る
      </Link>

      <div className={`grid gap-6 ${isOwner && !isEditing ? "lg:grid-cols-[minmax(0,1fr)_280px]" : ""}`}>
        <article className="rounded-2xl border border-white/[0.08] bg-white/[0.04] p-6 sm:p-10">
          <div className="mb-8 flex flex-wrap justify-between gap-3 border-b border-white/[0.08] pb-4 text-xs uppercase tracking-[0.12em] text-[#5a5a6e]">
            <span>POST / {post.id}</span>
            <span>{post.author?.username ?? "unknown"}</span>
          </div>
          {isEditing ? (
            <form className="space-y-5" onSubmit={handleUpdate}>
              <label className="block">
                <span className="mb-2 block text-xs font-semibold uppercase tracking-[0.08em] text-[#8b8b9e]">タイトル</span>
                <input
                  className="w-full rounded-xl border border-white/[0.08] bg-white/[0.04] px-4 py-3 text-sm text-[#f0f0f5] outline-none transition-all focus:border-[#6c63ff] focus:ring-[3px] focus:ring-[#6c63ff]/25"
                  value={title}
                  onChange={(event) => setTitle(event.target.value)}
                  maxLength={200}
                  required
                />
              </label>
              <label className="block">
                <span className="mb-2 block text-xs font-semibold uppercase tracking-[0.08em] text-[#8b8b9e]">本文</span>
                <textarea
                  className="min-h-48 w-full resize-y rounded-xl border border-white/[0.08] bg-white/[0.04] px-4 py-3 text-sm leading-[1.7] text-[#f0f0f5] outline-none transition-all focus:border-[#6c63ff] focus:ring-[3px] focus:ring-[#6c63ff]/25"
                  value={content}
                  onChange={(event) => setContent(event.target.value)}
                  required
                />
              </label>
              {error && <p className="text-sm text-[#f87171]" role="alert">{error}</p>}
              <div className="flex flex-wrap justify-end gap-3">
                <button
                  className="inline-flex items-center justify-center rounded-xl border border-white/[0.08] bg-white/[0.06] px-5 py-3 text-sm font-semibold text-[#f0f0f5] transition-colors hover:bg-white/[0.1]"
                  type="button"
                  onClick={() => setIsEditing(false)}
                >
                  キャンセル
                </button>
                <button
                  className="inline-flex items-center justify-center gap-2 rounded-xl bg-gradient-to-br from-[#6c63ff] to-[#8b7bff] px-5 py-3 text-sm font-semibold text-white shadow-[0_4px_20px_rgba(108,99,255,0.25)] disabled:cursor-not-allowed disabled:opacity-50"
                  type="submit"
                  disabled={updatePost.isPending}
                >
                  {updatePost.isPending ? "保存中…" : "変更を保存"} ↗
                </button>
              </div>
            </form>
          ) : (
            <>
              <h1 className="text-3xl font-extrabold leading-[1.25] tracking-[-0.03em] sm:text-4xl">{post.title}</h1>
              <div className="mt-8 whitespace-pre-wrap break-words text-base leading-[1.9] text-[#c7c7d0]">{post.content}</div>
              <div className="mt-10 flex flex-wrap gap-4 border-t border-white/[0.08] pt-4 text-xs text-[#5a5a6e]">
                <span>Created {formatLongDate(post.createdAt)}</span>
                {post.updatedAt && post.updatedAt !== post.createdAt && <span>Updated {formatLongDate(post.updatedAt)}</span>}
              </div>
            </>
          )}
        </article>

        {isOwner && !isEditing && (
          <aside className="space-y-3">
            <button
              className="w-full rounded-xl border border-white/[0.08] bg-white/[0.06] px-5 py-3 text-sm font-semibold text-[#f0f0f5] transition-colors hover:bg-white/[0.1]"
              type="button"
              onClick={() => {
                setTitle(post.title ?? "");
                setContent(post.content ?? "");
                setIsEditing(true);
              }}
            >
              投稿を編集
            </button>
            <button className="w-full text-xs text-[#f87171] transition-opacity hover:opacity-80 disabled:opacity-50" type="button" onClick={handleDelete} disabled={deletePost.isPending}>
              {deletePost.isPending ? "削除中…" : "この投稿を削除する"}
            </button>
            {error && <p className="text-sm text-[#f87171]" role="alert">{error}</p>}
          </aside>
        )}
      </div>
    </div>
  );
}
