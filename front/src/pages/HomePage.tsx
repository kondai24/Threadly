import { Link } from "react-router-dom";

export default function HomePage() {
  return (
    <div>
      <section className="hero">
        <div className="hero-badge">
          <span className="dot"></span>
          オープンソース掲示板アプリ
        </div>
        <h1>
          みんなの声を
          <br />
          ひとつの場所に
        </h1>
        <p>
          Threadyは、シンプルで美しい掲示板アプリケーションです。
          あなたのアイデアや意見を自由に投稿し、コミュニティと共有しましょう。
        </p>
        <div className="hero-actions">
          <Link to="/board" className="btn btn-primary">
            🚀 掲示板を見る
          </Link>
          <a
            href="https://github.com"
            target="_blank"
            rel="noopener noreferrer"
            className="btn btn-secondary"
          >
            GitHub
          </a>
        </div>
      </section>

      <div className="features-grid">
        <div className="feature-card">
          <div className="icon">📝</div>
          <h3>かんたん投稿</h3>
          <p>
            タイトルと本文を入力するだけで、すぐに投稿できます。煩雑な手続きは必要ありません。
          </p>
        </div>
        <div className="feature-card">
          <div className="icon">⚡</div>
          <h3>高速レスポンス</h3>
          <p>
            GoとReactで構築された高速なバックエンドとフロントエンドで、快適な体験を提供します。
          </p>
        </div>
        <div className="feature-card">
          <div className="icon">🎨</div>
          <h3>モダンなUI</h3>
          <p>
            ダークテーマとグラスモーフィズムを採用した、見やすく美しいインターフェースです。
          </p>
        </div>
      </div>
    </div>
  );
}
