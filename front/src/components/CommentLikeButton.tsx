import { useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import {
  useDeleteApiCommentsIdLike,
  usePutApiCommentsIdLike,
} from "../orval/threadyAPI";
import { updateCommentLikeCache } from "../lib/like-cache";
import LikeButton from "./LikeButton";

type CommentLikeButtonProps = {
  postId: string;
  commentId: string;
  likeCount?: number;
  likedByMe?: boolean;
};

function getLikeErrorMessage(error: unknown) {
  return error instanceof Error
    ? error.message
    : "いいねを更新できませんでした。";
}

export default function CommentLikeButton({
  postId,
  commentId,
  likeCount,
  likedByMe,
}: CommentLikeButtonProps) {
  const queryClient = useQueryClient();
  const likeComment = usePutApiCommentsIdLike();
  const unlikeComment = useDeleteApiCommentsIdLike();
  const [error, setError] = useState<string | null>(null);

  const handleSuccess = (
    result: Parameters<typeof updateCommentLikeCache>[2],
  ) => {
    updateCommentLikeCache(queryClient, postId, result);
  };
  const handleError = (mutationError: unknown) => {
    setError(getLikeErrorMessage(mutationError));
  };
  const handleLike = () => {
    setError(null);
    likeComment.mutate(
      { id: commentId },
      { onSuccess: handleSuccess, onError: handleError },
    );
  };
  const handleUnlike = () => {
    setError(null);
    unlikeComment.mutate(
      { id: commentId },
      { onSuccess: handleSuccess, onError: handleError },
    );
  };

  return (
    <span className="inline-flex flex-col items-start gap-1">
      <LikeButton
        likeCount={likeCount}
        likedByMe={likedByMe}
        isPending={likeComment.isPending || unlikeComment.isPending}
        onLike={handleLike}
        onUnlike={handleUnlike}
      />
      {error && (
        <span className="text-[0.65rem] text-[#f87171]" role="alert">
          {error}
        </span>
      )}
    </span>
  );
}
