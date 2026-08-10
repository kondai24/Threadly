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
    <div className="home-page">
      <section className="home-hero">
        <div className="hero-copy">
          <span className="eyebrow">THREADLY / CONVERSATION STUDIO</span>
          <h1 className="display-title">
            会話の続きを、
            <em>置いておく。</em>
          </h1>
          <p className="hero-lede">
            Threadlyは、考えの断片を自分のボードに残しておくための小さな投稿空間です。
            書き始める場所と、戻ってくる場所をひとつに。
          </p>
          <div className="hero-actions">
            <Link to={isAuthenticated ? "/board" : "/register"} className="button button-primary">
              {isAuthenticated ? "ボードを開く" : "席をつくる"} <span aria-hidden="true">↗</span>
            </Link>
            {!isAuthenticated && (
              <Link to="/login" className="button button-secondary">
                ログイン
              </Link>
            )}
          </div>
        </div>

        <div className="hero-aside" aria-label="Threadlyの特徴">
          <div className="hero-signal-top">
            <span>LIVE THREAD</span>
            <span className="signal-dot" aria-hidden="true" />
          </div>
          <div className="signal-board">
            <div className="signal-line" aria-hidden="true" />
            <div className="signal-card signal-card-back">
              <span className="signal-card-label">03 / LATER</span>
              <strong>あの話の続きを書く</strong>
            </div>
            <div className="signal-card signal-card-front">
              <span className="signal-card-label">01 / NOW</span>
              <strong>今日の考えを一行で</strong>
              <span className="signal-card-meta">your board · just now</span>
            </div>
          </div>
          <p className="hero-aside-note">投稿が一枚ずつ、次の考えへの目印になる。</p>
        </div>
      </section>

      <div className="section-rule">
        <span>Why Threadly</span>
        <span>01 — 03</span>
      </div>

      <section className="principles-grid">
        {principles.map((principle) => (
          <article className="principle-card" key={principle.label}>
            <span className="card-label">{principle.label}</span>
            <h2>{principle.title}</h2>
            <p>{principle.text}</p>
          </article>
        ))}
      </section>

      <footer className="home-footer-note">
        <span className="footer-line" aria-hidden="true" />
        <p>自分の言葉を、自分のペースで。Threadly / built for continuing thoughts.</p>
      </footer>
    </div>
  );
}
