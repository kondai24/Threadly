import { useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import {
  useDeleteApiPostsIdLike,
  usePutApiPostsIdLike,
} from "../orval/threadyAPI";
import LikeButton from "./LikeButton";
import { updatePostLikeCache } from "../lib/like-cache";

type PostLikeButtonProps = {
  postId: string;
  likeCount?: number;
  likedByMe?: boolean;
};

function getLikeErrorMessage(error: unknown) {
  return error instanceof Error
    ? error.message
    : "いいねを更新できませんでした。";
}

export default function PostLikeButton({
  postId,
  likeCount,
  likedByMe,
}: PostLikeButtonProps) {
  const queryClient = useQueryClient();
  const likePost = usePutApiPostsIdLike();
  const unlikePost = useDeleteApiPostsIdLike();
  const [error, setError] = useState<string | null>(null);

  const handleSuccess = (result: Parameters<typeof updatePostLikeCache>[1]) => {
    updatePostLikeCache(queryClient, result);
  };
  const handleError = (mutationError: unknown) => {
    setError(getLikeErrorMessage(mutationError));
  };
  const handleLike = () => {
    setError(null);
    likePost.mutate(
      { id: postId },
      { onSuccess: handleSuccess, onError: handleError },
    );
  };
  const handleUnlike = () => {
    setError(null);
    unlikePost.mutate(
      { id: postId },
      { onSuccess: handleSuccess, onError: handleError },
    );
  };

  return (
    <span className="inline-flex flex-col items-end gap-1">
      <LikeButton
        likeCount={likeCount}
        likedByMe={likedByMe}
        isPending={likePost.isPending || unlikePost.isPending}
        onLike={handleLike}
        onUnlike={handleUnlike}
      />
      {error && (
        <span className="text-right text-[0.65rem] text-[#f87171]" role="alert">
          {error}
        </span>
      )}
    </span>
  );
}
