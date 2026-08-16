import { useState, type FormEvent } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { formatDate } from "../lib/format";
import type { InternalInterfaceControllersCommentResponse } from "../orval/threadyAPI.schemas";
import {
  getGetApiPostsIdCommentsQueryKey,
  useGetApiPostsIdComments,
  usePostApiPostsIdComments,
} from "../orval/threadyAPI";

const maxCommentLength = 1000;

function getErrorMessage(error: unknown, fallback: string) {
  return error instanceof Error ? error.message : fallback;
}

type CommentComposerProps = {
  postId: string;
  parentId?: string;
  onCancel?: () => void;
  onCreated?: () => void;
};

function CommentComposer({
  postId,
  parentId,
  onCancel,
  onCreated,
}: CommentComposerProps) {
  const queryClient = useQueryClient();
  const createComment = usePostApiPostsIdComments();
  const [content, setContent] = useState("");
  const [error, setError] = useState<string | null>(null);
  const isReply = parentId !== undefined;

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const nextContent = content.trim();
    if (!nextContent) {
      setError("コメントを入力してください。");
      return;
    }

    setError(null);
    createComment.mutate(
      {
        id: postId,
        data: isReply
          ? { content: nextContent, parentId }
          : { content: nextContent },
      },
      {
        onSuccess: async () => {
          setContent("");
          await queryClient.invalidateQueries({
            queryKey: getGetApiPostsIdCommentsQueryKey(postId),
          });
          onCreated?.();
        },
        onError: (mutationError) =>
          setError(
            getErrorMessage(
              mutationError,
              isReply
                ? "返信を投稿できませんでした。"
                : "コメントを投稿できませんでした。",
            ),
          ),
      },
    );
  };

  return (
    <form
      className={isReply ? "mt-4 space-y-3" : "mt-5 space-y-4"}
      onSubmit={handleSubmit}
    >
      <label className="block">
        <span className="mb-2 block text-xs font-semibold uppercase tracking-[0.08em] text-[#8b8b9e]">
          {isReply ? "返信" : "コメント"}
        </span>
        <textarea
          className="min-h-28 w-full resize-y rounded-xl border border-white/[0.08] bg-white/[0.04] px-4 py-3 text-sm leading-[1.7] text-[#f0f0f5] outline-none transition-all placeholder:text-[#5a5a6e] focus:border-[#6c63ff] focus:ring-[3px] focus:ring-[#6c63ff]/25"
          value={content}
          onChange={(event) => setContent(event.target.value)}
          placeholder={
            isReply ? "このコメントに返信する" : "この投稿についてコメントする"
          }
          maxLength={maxCommentLength}
          required
        />
      </label>
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-center gap-3 text-xs text-[#5a5a6e]">
          <span>
            {content.length.toLocaleString("ja-JP")} / {maxCommentLength}
          </span>
          {error && (
            <span className="text-[#f87171]" role="alert">
              {error}
            </span>
          )}
        </div>
        <div className="flex justify-end gap-3">
          {onCancel && (
            <button
              className="rounded-xl border border-white/[0.08] bg-white/[0.04] px-4 py-2.5 text-sm font-semibold text-[#c7c7d0] transition-colors hover:bg-white/[0.08]"
              type="button"
              onClick={onCancel}
            >
              キャンセル
            </button>
          )}
          <button
            className="inline-flex items-center justify-center gap-2 rounded-xl bg-gradient-to-br from-[#6c63ff] to-[#8b7bff] px-5 py-2.5 text-sm font-semibold text-white shadow-[0_4px_20px_rgba(108,99,255,0.25)] outline-none transition-all hover:-translate-y-0.5 focus-visible:ring-2 focus-visible:ring-[#6c63ff]/50 disabled:cursor-not-allowed disabled:opacity-50"
            type="submit"
            disabled={createComment.isPending || !content.trim()}
          >
            {createComment.isPending
              ? "投稿中…"
              : isReply
                ? "返信する"
                : "コメントする"}{" "}
            ↗
          </button>
        </div>
      </div>
    </form>
  );
}

type CommentCardProps = {
  postId: string;
  comment: InternalInterfaceControllersCommentResponse;
  isReply?: boolean;
};

function CommentCard({ postId, comment, isReply = false }: CommentCardProps) {
  const [isReplying, setIsReplying] = useState(false);
  const replies = comment.replies ?? [];
  const canReply = !isReply && comment.id !== undefined;
  const authorName = comment.author?.username ?? "unknown";
  const commentKey =
    comment.id ?? `${comment.createdAt ?? "comment"}-${authorName}`;

  return (
    <article
      className={
        isReply
          ? "rounded-xl border border-white/[0.06] bg-white/[0.025] p-4"
          : "rounded-2xl border border-white/[0.08] bg-white/[0.04] p-5"
      }
    >
      <div className="flex items-start gap-3">
        <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-[#6c63ff]/20 text-xs font-bold text-[#a79fff]">
          {authorName.slice(0, 1).toUpperCase()}
        </span>
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
            <strong className="text-sm text-[#f0f0f5]">{authorName}</strong>
            <span className="text-xs text-[#5a5a6e]">
              {formatDate(comment.createdAt)}
            </span>
          </div>
          <p className="mt-3 whitespace-pre-wrap break-words text-sm leading-[1.8] text-[#c7c7d0]">
            {comment.content}
          </p>
          {canReply && (
            <button
              className="mt-3 text-xs font-semibold text-[#8b7bff] transition-colors hover:text-[#b0a8ff]"
              type="button"
              onClick={() => setIsReplying((current) => !current)}
            >
              {isReplying ? "返信を閉じる" : "返信する"}
            </button>
          )}
          {isReplying && comment.id && (
            <CommentComposer
              postId={postId}
              parentId={comment.id}
              onCancel={() => setIsReplying(false)}
              onCreated={() => setIsReplying(false)}
            />
          )}
        </div>
      </div>

      {replies.length > 0 && (
        <div className="mt-4 space-y-3 border-l border-[#6c63ff]/30 pl-4 sm:ml-11">
          {replies.map((reply) => (
            <CommentCard
              key={
                reply.id ?? `${commentKey}-${reply.createdAt ?? reply.content}`
              }
              postId={postId}
              comment={reply}
              isReply
            />
          ))}
        </div>
      )}
    </article>
  );
}

export default function CommentsSection({ postId }: { postId: string }) {
  const {
    data: comments = [],
    error,
    isError,
    isPending,
    refetch,
  } = useGetApiPostsIdComments(postId);
  const commentCount = comments.reduce(
    (count, comment) => count + 1 + (comment.replies?.length ?? 0),
    0,
  );

  return (
    <section
      className="mt-8 border-t border-white/[0.08] pt-8"
      aria-labelledby="comments-title"
    >
      <div className="flex flex-wrap items-end justify-between gap-4 border-b border-white/[0.08] pb-4">
        <div>
          <span className="text-[0.65rem] font-semibold uppercase tracking-[0.16em] text-[#6c63ff]">
            Conversation / {commentCount}
          </span>
          <h2 id="comments-title" className="mt-2 text-2xl font-bold">
            コメント
          </h2>
        </div>
        <span className="text-xs text-[#5a5a6e]">Post / comments</span>
      </div>

      <CommentComposer postId={postId} />

      <div className="mt-8">
        {isPending ? (
          <div className="rounded-2xl border border-dashed border-white/[0.12] px-6 py-10 text-center text-sm text-[#8b8b9e]">
            コメントを読み込んでいます…
          </div>
        ) : isError ? (
          <div className="rounded-2xl border border-dashed border-white/[0.12] px-6 py-10 text-center">
            <p className="text-sm text-[#f87171]" role="alert">
              {getErrorMessage(error, "コメントを読み込めませんでした。")}
            </p>
            <button
              className="mt-4 rounded-xl border border-white/[0.08] bg-white/[0.06] px-4 py-2.5 text-sm font-semibold text-[#f0f0f5] transition-colors hover:bg-white/[0.1]"
              type="button"
              onClick={() => void refetch()}
            >
              再読み込み
            </button>
          </div>
        ) : comments.length === 0 ? (
          <div className="rounded-2xl border border-dashed border-white/[0.12] px-6 py-10 text-center">
            <p className="text-sm font-semibold text-[#c7c7d0]">
              まだコメントはありません。
            </p>
            <p className="mt-2 text-sm text-[#8b8b9e]">
              最初のコメントを残して会話を始めましょう。
            </p>
          </div>
        ) : (
          <div className="space-y-3">
            {comments.map((comment) => (
              <CommentCard
                key={
                  comment.id ??
                  `${comment.createdAt ?? "comment"}-${comment.content}`
                }
                postId={postId}
                comment={comment}
              />
            ))}
          </div>
        )}
      </div>
    </section>
  );
}
