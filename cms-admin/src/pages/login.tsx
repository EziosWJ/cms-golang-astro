import { useEffect, useState, type FormEvent } from "react";
import { Navigate, useLocation, useNavigate } from "react-router-dom";
import arrowRightIcon from "@/assets/login/icons/arrow-right.svg";
import eyeIcon from "@/assets/login/icons/eye.svg";
import featureReliableIcon from "@/assets/login/icons/feature-reliable.svg";
import featureRealtimeIcon from "@/assets/login/icons/feature-realtime.svg";
import featureUnifiedIcon from "@/assets/login/icons/feature-unified.svg";
import lockIcon from "@/assets/login/icons/lock.svg";
import logoMark from "@/assets/login/icons/logo-mark.svg";
import shieldCheckIcon from "@/assets/login/icons/shield-check.svg";
import userIcon from "@/assets/login/icons/user.svg";
import heroIllustration from "@/assets/login/hero/hero-cms-workflow.png";
import { Field } from "@/components/common/field";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { isApiError } from "@/lib/api-error";
import { useAuthStore } from "@/store/auth-store";
import "@/styles/login.css";
import type { LoginErrors } from "@/types";

const REMEMBERED_USERNAME_KEY = "cms-admin-remembered-username";

const BRAND_NAME = "CMS 内容管理";
const BRAND_DESC = "Intelligent Management Platform";

const FEATURES = [
  { icon: featureUnifiedIcon, title: "统一", description: "权限与数据视图" },
  { icon: featureRealtimeIcon, title: "实时", description: "关键状态可追踪" },
  { icon: featureReliableIcon, title: "可靠", description: "操作留痕可审计" },
] as const;

function BrandMark({ size }: { size: "md" | "lg" }) {
  return (
    <img
      src={logoMark}
      alt=""
      aria-hidden
      width={size === "lg" ? 64 : 44}
      height={size === "lg" ? 64 : 44}
      className={
        size === "lg"
          ? "login-brand__logo shrink-0 shadow-[0_12px_26px_rgba(22,119,255,0.24)]"
          : "h-11 w-11 shrink-0 rounded-[13px] shadow-[0_8px_20px_rgba(22,119,255,0.22)]"
      }
    />
  );
}

function MobileBrand() {
  return (
    <div className="mb-8 flex items-center gap-3 min-[992px]:hidden">
      <BrandMark size="md" />
      <div>
        <p className="text-base font-semibold tracking-tight text-[var(--cms-text)]">
          {BRAND_NAME}
        </p>
        <p className="mt-0.5 text-xs text-[var(--cms-text-muted)]">{BRAND_DESC}</p>
      </div>
    </div>
  );
}

export function LoginPage() {
  const navigate = useNavigate();
  const location = useLocation();
  const login = useAuthStore((state) => state.login);
  const isAuthenticated = useAuthStore((state) => state.isAuthenticated);
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [rememberMe, setRememberMe] = useState(false);
  const [errors, setErrors] = useState<LoginErrors>({});
  const [submitting, setSubmitting] = useState(false);

  const stateFrom =
    (location.state as { from?: { pathname?: string } } | null)?.from
      ?.pathname;
  const queryRedirect = new URLSearchParams(location.search).get("redirect");
  let from = stateFrom ?? queryRedirect ?? "/content/articles";

  if (from.includes("://") || from.startsWith("//")) {
    from = "/content/articles";
  }

  useEffect(() => {
    try {
      const rememberedUsername = localStorage.getItem(REMEMBERED_USERNAME_KEY);
      if (rememberedUsername) {
        setUsername(rememberedUsername);
        setRememberMe(true);
      }
    } catch {
      // 浏览器禁用本地存储时不影响正常登录。
    }
  }, []);

  if (isAuthenticated) {
    return <Navigate to={from} replace />;
  }

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const nextErrors: LoginErrors = {};

    if (!username.trim()) {
      nextErrors.username = "请输入用户名";
    }
    if (!password) {
      nextErrors.password = "请输入密码";
    }

    if (Object.keys(nextErrors).length > 0) {
      setErrors(nextErrors);
      return;
    }

    try {
      setSubmitting(true);
      await login(username.trim(), password);
      try {
        if (rememberMe) {
          localStorage.setItem(REMEMBERED_USERNAME_KEY, username.trim());
        } else {
          localStorage.removeItem(REMEMBERED_USERNAME_KEY);
        }
      } catch {
        // 本地存储不可用时仍以登录结果为准。
      }
      navigate(from, { replace: true });
    } catch (error) {
      setErrors({
        account: isApiError(error)
          ? error.message
          : "登录失败，请稍后重试",
      });
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <main className="login-page flex min-h-screen w-full min-w-0 flex-col overflow-x-hidden bg-[var(--cms-bg)] min-[992px]:grid min-[992px]:h-screen min-[992px]:grid-cols-[55%_45%] min-[992px]:overflow-hidden">
      <section
        className="login-brand relative hidden overflow-hidden px-10 py-10 min-[992px]:flex min-[992px]:flex-col xl:px-16 2xl:px-24"
        aria-label="CMS 内容管理介绍"
      >
        <div className="login-brand__grid" aria-hidden />
        <div className="login-brand__wave" aria-hidden />
        <div className="login-brand__orbit" aria-hidden />
        <div
          className="login-brand__arc right-[6%] top-[16%] h-[78px] w-[78px]"
          aria-hidden
        />
        <div
          className="login-brand__arc bottom-[22%] left-[3%] h-16 w-16"
          aria-hidden
        />

        <div className="relative z-10 flex min-h-0 w-full flex-1 flex-col">
          <div className="flex items-center gap-4">
            <BrandMark size="lg" />
            <div>
              <p className="login-brand__name font-semibold tracking-tight text-[var(--cms-text)]">
                {BRAND_NAME}
              </p>
              <p className="login-brand__name-desc mt-0.5 text-[var(--cms-text-muted)]">
                {BRAND_DESC}
              </p>
            </div>
          </div>

          <div className="mt-10 max-w-[620px] xl:mt-14">
            <p className="login-brand__kicker font-medium text-[var(--cms-brand)]">
              写作、预览与发布你的博客
            </p>
            <h2 className="login-brand__title mt-3 font-bold text-[var(--cms-text)]">
              安全 · 稳定 · 高效
            </h2>
            <p className="login-brand__subtitle mt-4 max-w-[520px] leading-7 text-[var(--cms-text-muted)]">
              统一权限、业务数据与运营流程，让管理工作清晰可控。
            </p>
          </div>

          <div className="login-hero py-6">
            <img
              src={heroIllustration}
              alt="内容工作流界面示意图"
              width={1200}
              height={760}
            />
          </div>

          <ul className="mt-2 hidden max-w-[760px] grid-cols-3 min-[1280px]:grid">
            {FEATURES.map((feature, index) => (
              <li
                key={feature.title}
                className={
                  index === 0
                    ? "flex items-center gap-3 pr-6"
                    : "flex items-center gap-3 border-l border-[rgba(124,180,255,0.32)] px-6"
                }
              >
                <img
                  src={feature.icon}
                  alt=""
                  aria-hidden
                  width={40}
                  height={40}
                  className="login-brand__feature-icon shrink-0"
                />
                <div className="min-w-0">
                  <p className="login-brand__feature-title font-semibold text-[var(--cms-text)]">
                    {feature.title}
                  </p>
                  <p className="login-brand__feature-desc mt-0.5 truncate text-[var(--cms-text-muted)]">
                    {feature.description}
                  </p>
                </div>
              </li>
            ))}
          </ul>
        </div>
      </section>

      <section className="login-panel flex w-full flex-1 flex-col overflow-y-auto px-5 py-10 sm:px-8 min-[992px]:min-h-0 min-[992px]:px-10">
        <div className="login-card m-auto w-full">
          <div className="login-card__body rounded-[var(--cms-radius-card)] bg-[var(--cms-card)] shadow-[var(--cms-shadow-card)]">
            <MobileBrand />

            <div className="mb-9">
              <p className="login-card__kicker font-medium text-[var(--cms-brand)]">
                欢迎回来
              </p>
              <h1 className="login-card__title mt-2 font-bold tracking-tight text-[var(--cms-text)]">
                欢迎登录
              </h1>
              <p className="login-card__subtitle mt-2 text-[var(--cms-text-muted)]">
                请输入账号信息进入系统
              </p>
            </div>

            <form className="space-y-5" onSubmit={handleSubmit}>
              <Field
                label="用户名"
                htmlFor="username"
                required
                error={errors.username}
              >
                <div className="relative">
                  <img
                    src={userIcon}
                    alt=""
                    aria-hidden
                    className="login-field-icon pointer-events-none absolute top-1/2 h-5 w-5 -translate-y-1/2"
                  />
                  <Input
                    id="username"
                    value={username}
                    placeholder="请输入用户名"
                    onChange={(event) => {
                      setUsername(event.target.value);
                      setErrors((current) => ({ ...current, username: undefined, account: undefined }));
                    }}
                    className="login-input text-sm"
                    autoComplete="username"
                    aria-invalid={Boolean(errors.username)}
                  />
                </div>
              </Field>

              <Field
                label="密码"
                htmlFor="password"
                required
                error={errors.password}
              >
                <div className="relative">
                  <img
                    src={lockIcon}
                    alt=""
                    aria-hidden
                    className="login-field-icon pointer-events-none absolute top-1/2 h-5 w-5 -translate-y-1/2"
                  />
                  <Input
                    id="password"
                    type={showPassword ? "text" : "password"}
                    value={password}
                    placeholder="请输入密码"
                    onChange={(event) => {
                      setPassword(event.target.value);
                      setErrors((current) => ({ ...current, password: undefined, account: undefined }));
                    }}
                    className="login-input text-sm"
                    autoComplete="current-password"
                    aria-invalid={Boolean(errors.password)}
                  />
                  <button
                    type="button"
                    className="login-eye-button absolute top-1/2 -translate-y-1/2 rounded-md p-1.5 text-[var(--cms-text-muted)] transition-colors hover:text-[var(--cms-text)]"
                    aria-label={showPassword ? "隐藏密码" : "显示密码"}
                    onClick={() => setShowPassword((value) => !value)}
                  >
                    <span
                      className={
                        showPassword ? "login-eye login-eye--off" : "login-eye"
                      }
                      aria-hidden
                    >
                      <img src={eyeIcon} alt="" className="h-[18px] w-[18px]" />
                    </span>
                  </button>
                </div>
              </Field>

              <div className="flex items-center justify-between pt-2">
                <label
                  htmlFor="remember-me"
                  className="inline-flex cursor-pointer items-center gap-2 text-sm text-[var(--cms-text)]"
                >
                  <Checkbox
                    id="remember-me"
                    className="login-checkbox"
                    checked={rememberMe}
                    onChange={(event) => setRememberMe(event.target.checked)}
                  />
                  记住我
                </label>
                <span className="inline-flex items-center gap-1.5 text-sm text-[var(--cms-text-muted)]">
                  <img
                    src={shieldCheckIcon}
                    alt=""
                    aria-hidden
                    className="h-[18px] w-[18px]"
                  />
                  安全登录
                </span>
              </div>

              {errors.account && (
                <p
                  className="rounded-[var(--cms-radius-control)] border border-[var(--color-error-border)] bg-[var(--color-error-background)] px-3.5 py-3 text-sm leading-5 text-error"
                  role="alert"
                >
                  {errors.account}
                </p>
              )}

              <Button
                type="submit"
                variant="primary"
                size="lg"
                className="login-submit mt-1 w-full"
                disabled={submitting}
                aria-busy={submitting}
              >
                {submitting ? (
                  <>
                    <span
                      className="h-4 w-4 animate-spin rounded-full border-2 border-white/40 border-t-white"
                      aria-hidden
                    />
                    登录中…
                  </>
                ) : (
                  <>
                    登录
                    <img
                      src={arrowRightIcon}
                      alt=""
                      aria-hidden
                      className="h-[18px] w-[18px]"
                    />
                  </>
                )}
              </Button>
            </form>
          </div>

          <p className="mt-6 text-center text-xs text-[var(--cms-text-muted)]">
            © 2026 CMS 内容管理
          </p>
        </div>
      </section>
    </main>
  );
}
