import { Link } from "react-router-dom";
import { useAuth } from "../lib/auth-context";

const principles = [
  {
    label: "A / Archive",
    title: "思考を置いておく",
    text: "ふと思いついたことも、あとで続きを書ける自分のボードへ。",
  },
  {
    label: "B / Signal",
    title: "流れを見失わない",
    text: "投稿は時間と作者の情報をまとい、会話の流れとして並びます。",
  },
  {
    label: "C / Ownership",
    title: "自分の言葉を管理する",
    text: "投稿はみんなで読めて、編集と削除は書いた本人だけ。境界が明快です。",
  },
];

export default function HomePage() {
  const { isAuthenticated } = useAuth();

  return (
    <div className="space-y-16">
      <section className="grid gap-12 py-8 lg:grid-cols-[1.15fr_0.85fr] lg:items-center lg:py-16">
        <div>
          <span className="text-xs font-semibold uppercase tracking-[0.16em] text-[#8b8b9e]">
            THREADLY / CONVERSATION STUDIO
          </span>
          <h1 className="mt-6 max-w-2xl text-[clamp(2.5rem,6vw,5rem)] font-black leading-[1.08] tracking-[-0.04em] text-[#f0f0f5]">
            会話の続きを、
            <em className="block bg-gradient-to-r from-[#6c63ff] to-[#ff6b9d] bg-clip-text not-italic text-transparent">
              置いておく。
            </em>
          </h1>
          <p className="mt-6 max-w-xl text-base leading-[1.8] text-[#8b8b9e]">
            Threadlyは、考えの断片を自分のボードに残しておくための小さな投稿空間です。
            書き始める場所と、戻ってくる場所をひとつに。
          </p>
          <div className="mt-8 flex flex-wrap gap-3">
            <Link
              to={isAuthenticated ? "/board" : "/register"}
              className="inline-flex items-center justify-center gap-2 rounded-xl bg-gradient-to-br from-[#6c63ff] to-[#8b7bff] px-6 py-3 text-[0.9rem] font-semibold text-white no-underline shadow-[0_4px_20px_rgba(108,99,255,0.25)] outline-none transition-all duration-200 hover:-translate-y-0.5 hover:shadow-[0_6px_30px_rgba(108,99,255,0.25)] focus-visible:ring-2 focus-visible:ring-[#6c63ff]/50"
            >
              {isAuthenticated ? "ボードを開く" : "席をつくる"} ↗
            </Link>
            {!isAuthenticated && (
              <Link
                to="/login"
                className="inline-flex items-center justify-center gap-2 rounded-xl border border-white/[0.08] bg-white/[0.06] px-6 py-3 text-[0.9rem] font-semibold text-[#f0f0f5] no-underline outline-none transition-all duration-200 hover:border-white/[0.15] hover:bg-white/[0.07] focus-visible:ring-2 focus-visible:ring-[#6c63ff]/50"
              >
                ログイン
              </Link>
            )}
          </div>
        </div>

        <div className="relative min-h-[300px] rounded-3xl border border-white/[0.08] bg-white/[0.04] p-6 sm:p-10">
          <div className="flex items-center justify-between text-[0.65rem] font-semibold uppercase tracking-[0.16em] text-[#8b8b9e]">
            <span>LIVE THREAD</span>
            <span className="h-2 w-2 animate-pulse rounded-full bg-[#4ade80]" />
          </div>
          <div className="absolute left-10 top-24 h-40 border-l border-dashed border-[#6c63ff]/50" aria-hidden="true" />
          <div className="absolute left-16 top-20 w-[calc(100%-5rem)] rotate-[-3deg] rounded-2xl border border-white/[0.08] bg-[#12121a] p-5 text-[#f0f0f5] shadow-[0_8px_40px_rgba(0,0,0,0.35)]">
            <span className="text-[0.65rem] font-semibold uppercase tracking-[0.14em] text-[#6c63ff]">03 / LATER</span>
            <strong className="mt-3 block text-lg">あの話の続きを書く</strong>
          </div>
          <div className="absolute bottom-8 right-8 w-[calc(100%-5rem)] rotate-[3deg] rounded-2xl border border-[#6c63ff]/40 bg-gradient-to-br from-[#1a1830] to-[#12121a] p-5 text-[#f0f0f5] shadow-[0_8px_40px_rgba(0,0,0,0.5)]">
            <span className="text-[0.65rem] font-semibold uppercase tracking-[0.14em] text-[#ff6b9d]">01 / NOW</span>
            <strong className="mt-3 block text-lg">今日の考えを一行で</strong>
            <span className="mt-2 block text-xs text-[#8b8b9e]">your board · just now</span>
          </div>
        </div>
      </section>

      <div className="flex items-center justify-between border-t border-white/[0.08] pt-4 text-xs font-semibold uppercase tracking-[0.14em] text-[#5a5a6e]">
        <span>Why Threadly</span>
        <span>01 — 03</span>
      </div>

      <section className="grid gap-4 md:grid-cols-3">
        {principles.map((principle) => (
          <article
            className="rounded-2xl border border-white/[0.08] bg-white/[0.04] p-6 transition-all duration-200 hover:-translate-y-0.5 hover:border-white/[0.15] hover:bg-white/[0.07]"
            key={principle.label}
          >
            <span className="text-[0.65rem] font-semibold uppercase tracking-[0.16em] text-[#6c63ff]">
              {principle.label}
            </span>
            <h2 className="mt-5 text-xl font-bold tracking-[-0.02em]">{principle.title}</h2>
            <p className="mt-3 text-sm leading-[1.7] text-[#8b8b9e]">{principle.text}</p>
          </article>
        ))}
      </section>

      <footer className="border-t border-white/[0.08] pt-4 text-sm text-[#5a5a6e]">
        自分の言葉を、自分のペースで。Threadly / built for continuing thoughts.
      </footer>
    </div>
  );
}
