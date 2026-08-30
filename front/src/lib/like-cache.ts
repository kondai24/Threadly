import type { QueryClient } from "@tanstack/react-query";
import {
  getGetApiPostsIdCommentsQueryKey,
  getGetApiPostsIdQueryKey,
  getGetApiPostsQueryKey,
} from "../orval/threadyAPI";
import type {
  ThreadlyInternalInterfaceDtoCommentResponse,
  ThreadlyInternalInterfaceDtoLikeActionResponse,
  ThreadlyInternalInterfaceDtoPostDetailResponse,
  ThreadlyInternalInterfaceDtoPostListResponse,
} from "../orval/threadyAPI.schemas";

function getLikeSummary(
  result: ThreadlyInternalInterfaceDtoLikeActionResponse,
) {
  return {
    likeCount: result.likeCount ?? 0,
    likedByMe: result.likedByMe ?? false,
  };
}

export function updatePostLikeCache(
  queryClient: QueryClient,
  result: ThreadlyInternalInterfaceDtoLikeActionResponse,
) {
  const targetId = result.targetId;
  if (!targetId) return;

  const summary = getLikeSummary(result);
  queryClient.setQueryData<ThreadlyInternalInterfaceDtoPostListResponse[]>(
    getGetApiPostsQueryKey(),
    (posts) =>
      posts?.map((post) =>
        post.id === targetId ? { ...post, ...summary } : post,
      ),
  );
  queryClient.setQueryData<ThreadlyInternalInterfaceDtoPostDetailResponse>(
    getGetApiPostsIdQueryKey(targetId),
    (post) => (post ? { ...post, ...summary } : post),
  );
}

function updateCommentLike(
  comment: ThreadlyInternalInterfaceDtoCommentResponse,
  targetId: string,
  summary: ReturnType<typeof getLikeSummary>,
): ThreadlyInternalInterfaceDtoCommentResponse {
  return {
    ...comment,
    ...(comment.id === targetId ? summary : {}),
    replies: comment.replies?.map((reply) =>
      reply.id === targetId ? { ...reply, ...summary } : reply,
    ),
  };
}

export function updateCommentLikeCache(
  queryClient: QueryClient,
  postId: string,
  result: ThreadlyInternalInterfaceDtoLikeActionResponse,
) {
  const targetId = result.targetId;
  if (!targetId) return;

  const summary = getLikeSummary(result);
  queryClient.setQueryData<ThreadlyInternalInterfaceDtoCommentResponse[]>(
    getGetApiPostsIdCommentsQueryKey(postId),
    (comments) =>
      comments?.map((comment) => updateCommentLike(comment, targetId, summary)),
  );
}
