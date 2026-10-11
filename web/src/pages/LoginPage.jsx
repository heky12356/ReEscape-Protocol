import { useState } from "react";
import { loginAdmin } from "../api/adminApi";

export function LoginPage({ onAuthenticated }) {
  const [key, setKey] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");

  async function handleSubmit(event) {
    event.preventDefault();
    const value = key.trim();
    if (!value) {
      setError("请输入管理密钥");
      return;
    }

    setSubmitting(true);
    setError("");
    try {
      await loginAdmin(value);
      setKey("");
      onAuthenticated();
    } catch (err) {
      setError(err?.message || "登录失败，请检查管理密钥");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <main className="auth-shell">
      <section className="auth-panel" aria-labelledby="login-title">
        <div className="brand auth-brand">
          <div className="brand-mark">
            <span className="brand-mark-core" />
          </div>
          <div>
            <div className="brand-title">ReEscape</div>
            <div className="brand-subtitle">管理控制台</div>
          </div>
        </div>

        <div className="auth-heading">
          <div className="eyebrow">Secure access</div>
          <h1 id="login-title">登录管理控制台</h1>
          <p>使用部署环境中配置的管理密钥建立临时会话。</p>
        </div>

        <form className="auth-form" onSubmit={handleSubmit}>
          <label className="field">
            <span className="field-label">管理密钥</span>
            <input
              className="field-input"
              type="password"
              value={key}
              onChange={(event) => setKey(event.target.value)}
              autoComplete="current-password"
              autoFocus
              spellCheck="false"
              disabled={submitting}
            />
          </label>
          {error ? <div className="auth-error" role="alert">{error}</div> : null}
          <button className="btn-primary auth-submit" type="submit" disabled={submitting}>
            {submitting ? "登录中..." : "登录"}
          </button>
        </form>
      </section>
    </main>
  );
}

export function AuthLoadingPage() {
  return (
    <main className="auth-shell">
      <section className="auth-panel auth-loading" aria-live="polite">
        <div className="eyebrow">Secure access</div>
        <p>正在检查登录状态...</p>
      </section>
    </main>
  );
}
