type LikeButtonProps = {
  likeCount?: number;
  likedByMe?: boolean;
  isPending?: boolean;
  onLike: () => void;
  onUnlike: () => void;
};

export default function LikeButton({
  likeCount = 0,
  likedByMe = false,
  isPending = false,
  onLike,
  onUnlike,
}: LikeButtonProps) {
  const label = likedByMe ? "いいねを取り消す" : "いいねする";

  return (
    <button
      className={`inline-flex items-center gap-1.5 rounded-full border px-3 py-1.5 text-xs font-semibold transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#6c63ff]/50 disabled:cursor-wait disabled:opacity-60 ${
        likedByMe
          ? "border-[#f472b6]/40 bg-[#f472b6]/10 text-[#f9a8d4]"
          : "border-white/[0.1] bg-white/[0.04] text-[#8b8b9e] hover:border-[#f472b6]/30 hover:text-[#f9a8d4]"
      }`}
      type="button"
      aria-label={label}
      aria-pressed={likedByMe}
      aria-busy={isPending}
      disabled={isPending}
      title={label}
      onClick={likedByMe ? onUnlike : onLike}
    >
      <span className="text-sm leading-none" aria-hidden="true">
        {likedByMe ? "♥" : "♡"}
      </span>
      <span>{likeCount.toLocaleString("ja-JP")}</span>
    </button>
  );
}
