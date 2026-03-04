import { Link, Navigate, useParams } from "react-router-dom";
import { useGetApiPostsIdSuspense } from "../orval/threadyAPI";

export default function PostDetailPage() {
  const { id } = useParams<{ id: string }>();
  const postId = id !== undefined ? Number(id) : NaN;
  if (id === undefined || Number.isNaN(postId)) {
    return <Navigate to="/board" replace />;
  }

  const { data: post } = useGetApiPostsIdSuspense(postId);

  const formatDate = (dateStr?: string) => {
    if (!dateStr) return "";
    try {
      return new Date(dateStr).toLocaleString("ja-JP", {
        year: "numeric",
        month: "long",
        day: "numeric",
        hour: "2-digit",
        minute: "2-digit",
      });
    } catch {
      return dateStr;
    }
  };

  return (
    <div className="post-detail">
      <div className="post-detail-header">
        <Link to="/board" className="back-link">
          ← 掲示板に戻る
        </Link>
        <h1>{post.title}</h1>
        <div className="meta">
          <span>#{post.id}</span>
          {post.createdAt && <span>投稿日: {formatDate(post.createdAt)}</span>}
          {post.updatedAt && post.updatedAt !== post.createdAt && (
            <span>更新日: {formatDate(post.updatedAt)}</span>
          )}
        </div>
      </div>
      <div className="content-body">{post.content}</div>
    </div>
  );
}
