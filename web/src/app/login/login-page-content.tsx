"use client";

import { useState } from "react";
import Link from "next/link";
import { useSearchParams, useRouter } from "next/navigation";
import { Header } from "@/components/layout/header";
import { getMe, startEmailLogin, verifyEmailLogin } from "@/lib/api";
import { getPostLoginPath } from "@/lib/auth-flow";
import { OAUTH_PROVIDERS } from "@/lib/oauth-providers";

type Step = "email" | "code";

export default function LoginPageContent() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const initialEmail = searchParams.get("email") ?? "";
  const [step, setStep] = useState<Step>(
    initialEmail && searchParams.get("start") === "true" ? "code" : "email"
  );
  const [email, setEmail] = useState(initialEmail);
  const [code, setCode] = useState("");
  const [loading, setLoading] = useState(false);
  const [oauthLoading, setOAuthLoading] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [devCode, setDevCode] = useState<string | null>(null);

  const startLogin = async (emailToUse: string) => {
    setError(null);
    setLoading(true);
    try {
      const res = await startEmailLogin(emailToUse);
      setStep("code");
      if (res.dev_code) {
        setDevCode(res.dev_code);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to send code");
    } finally {
      setLoading(false);
    }
  };

  const handleStartLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    startLogin(email);
  };

  const handleVerifyLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setLoading(true);
    try {
      const res = await verifyEmailLogin(email, code);
      router.push(getPostLoginPath(res.user));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Verification failed");
    } finally {
      setLoading(false);
    }
  };

  const handleBackToEmail = () => {
    setStep("email");
    setCode("");
    setDevCode(null);
    setError(null);
  };

  const handleOAuthLogin = (provider: (typeof OAUTH_PROVIDERS)[number]) => {
    if (!provider.loginHref || typeof window === "undefined") {
      return;
    }

    setError(null);
    setOAuthLoading(provider.id);

    const popup = window.open(
      provider.loginHref,
      `tokenlive-oauth-${provider.id}`,
      "width=520,height=680,menubar=no,toolbar=no,location=yes,status=no"
    );
    if (!popup) {
      setOAuthLoading(null);
      setError("Popup was blocked. Please allow popups and try again.");
      return;
    }

    const expectedOrigin = new URL(provider.loginHref, window.location.href).origin;
    const popupTimer = window.setInterval(() => {
      if (popup.closed) {
        cleanup();
      }
    }, 500);
    const cleanup = () => {
      window.clearInterval(popupTimer);
      window.removeEventListener("message", handleMessage);
      setOAuthLoading(null);
    };

    async function handleMessage(event: MessageEvent) {
      if (event.origin !== expectedOrigin) {
        return;
      }
      const data = event.data as {
        type?: string;
        success?: boolean;
        code?: string;
      };
      if (!data || data.type !== "oauth-callback") {
        return;
      }

      cleanup();
      if (!data.success) {
        setError(data.code || "OAuth sign-in failed");
        return;
      }

      try {
        const user = await getMe();
        router.push(getPostLoginPath(user));
      } catch (err) {
        setError(err instanceof Error ? err.message : "OAuth sign-in failed");
      }
    }

    window.addEventListener("message", handleMessage);
    popup.focus();
  };

  return (
    <>
      <Header />
      <main className="flex flex-1 items-center justify-center px-4 py-12">
        <div className="relative w-full max-w-sm">
          {/* Ambient glow */}
          <div className="pointer-events-none absolute -top-24 left-1/2 h-48 w-72 -translate-x-1/2 rounded-full bg-primary/8 blur-2xl" />

          <div className="relative rounded-xl border border-border bg-card p-6 shadow-lg">
            {/* Title */}
            <div className="mb-6 text-center">
              <div className="mx-auto mb-3 flex items-center justify-center gap-2">
                <span className="tl-node tl-node--lg tl-node--active" />
                <span className="font-heading text-lg font-semibold">
                  TokenLive
                </span>
              </div>
              <p className="text-sm text-muted-foreground">
                {step === "email"
                  ? "Sign in to your account"
                  : "Enter the verification code"}
              </p>
            </div>

            {step === "email" ? (
              /* Email step */
              <form onSubmit={handleStartLogin} className="space-y-4">
                <div>
                  <label
                    htmlFor="email"
                    className="block text-sm font-medium text-foreground mb-1.5"
                  >
                    Email address
                  </label>
                  <input
                    id="email"
                    type="email"
                    required
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                    placeholder="you@example.com"
                    className="h-10 w-full rounded-md border border-input bg-background px-3 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-primary/40"
                    autoFocus
                  />
                </div>

                {error && (
                  <p className="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
                    {error}
                  </p>
                )}

                <button
                  type="submit"
                  disabled={loading || !email}
                  className="h-10 w-full rounded-md bg-primary text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90 disabled:opacity-50 tl-focus-ring"
                >
                  {loading ? "Sending code..." : "Send code"}
                </button>
              </form>
            ) : (
              /* Code verification step */
              <form onSubmit={handleVerifyLogin} className="space-y-4">
                <div>
                  <label
                    htmlFor="code"
                    className="block text-sm font-medium text-foreground mb-1.5"
                  >
                    Verification code
                  </label>
                  <input
                    id="code"
                    type="text"
                    inputMode="numeric"
                    maxLength={6}
                    required
                    value={code}
                    onChange={(e) =>
                      setCode(e.target.value.replace(/\D/g, ""))
                    }
                    placeholder="000000"
                    className="h-10 w-full rounded-md border border-input bg-background px-3 font-mono text-center text-lg tracking-widest text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-primary/40"
                    autoFocus
                  />
                  <p className="mt-1.5 text-xs text-muted-foreground">
                    Sent to{" "}
                    <span className="font-medium text-foreground">{email}</span>
                  </p>
                </div>

                {devCode && (
                  <div className="rounded-md bg-primary/10 px-3 py-2">
                    <p className="text-xs font-medium text-primary">
                      Dev mode — your code is:{" "}
                      <span className="font-mono text-sm">{devCode}</span>
                    </p>
                  </div>
                )}

                {error && (
                  <p className="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
                    {error}
                  </p>
                )}

                <button
                  type="submit"
                  disabled={loading || code.length !== 6}
                  className="h-10 w-full rounded-md bg-primary text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90 disabled:opacity-50 tl-focus-ring"
                >
                  {loading ? "Verifying..." : "Verify & sign in"}
                </button>

                <button
                  type="button"
                  onClick={handleBackToEmail}
                  className="h-9 w-full rounded-md border border-border text-sm text-muted-foreground transition-colors hover:text-foreground hover:bg-secondary"
                >
                  Use a different email
                </button>
              </form>
            )}

            {/* Divider (only on email step) */}
            {step === "email" && (
              <>
                <div className="relative my-5">
                  <div className="absolute inset-0 flex items-center">
                    <div className="w-full border-t border-border" />
                  </div>
                  <div className="relative flex justify-center text-xs">
                    <span className="bg-card px-2 text-muted-foreground">or</span>
                  </div>
                </div>

                <div className="space-y-2">
                  {OAUTH_PROVIDERS.map((provider) =>
                    provider.enabled && provider.loginHref ? (
                      <button
                        key={provider.id}
                        type="button"
                        onClick={() => handleOAuthLogin(provider)}
                        disabled={oauthLoading !== null}
                        className="flex h-10 w-full items-center justify-center gap-2 rounded-md border border-border bg-background text-sm font-medium text-foreground transition-colors hover:bg-secondary tl-focus-ring"
                      >
                        <ProviderIcon provider={provider.id} />
                        {oauthLoading === provider.id
                          ? "Waiting for authentication..."
                          : `Continue with ${provider.label}`}
                      </button>
                    ) : (
                      <button
                        key={provider.id}
                        type="button"
                        disabled
                        className="flex h-10 w-full cursor-not-allowed items-center justify-center gap-2 rounded-md border border-border bg-background text-sm font-medium text-muted-foreground opacity-70"
                        title={provider.unavailableLabel || undefined}
                      >
                        <ProviderIcon provider={provider.id} />
                        {provider.label} unavailable
                      </button>
                    )
                  )}
                </div>
              </>
            )}

            {/* Register link (only on email step) */}
            {step === "email" && (
              <p className="mt-4 text-center text-sm text-muted-foreground">
                Don&apos;t have an account?{" "}
                <Link
                  href="/register"
                  className="text-foreground hover:text-primary transition-colors"
                >
                  Create one
                </Link>
              </p>
            )}
          </div>
        </div>
      </main>
    </>
  );
}

function ProviderIcon({ provider }: { provider: string }) {
  if (provider === "github") {
    return (
      <svg
        className="h-4 w-4"
        viewBox="0 0 24 24"
        fill="currentColor"
        aria-hidden="true"
      >
        <path d="M12 2C6.48 2 2 6.59 2 12.25c0 4.53 2.87 8.37 6.84 9.73.5.09.68-.22.68-.49 0-.24-.01-1.05-.01-1.9-2.51.47-3.16-.63-3.36-1.21-.11-.3-.6-1.21-1.03-1.46-.35-.19-.85-.66-.01-.67.79-.01 1.35.74 1.54 1.05.9 1.55 2.34 1.11 2.91.85.09-.67.35-1.11.64-1.37-2.22-.26-4.55-1.14-4.55-5.05 0-1.11.39-2.03 1.03-2.75-.1-.26-.45-1.3.1-2.71 0 0 .84-.27 2.75 1.05A9.3 9.3 0 0 1 12 6.98c.85 0 1.71.12 2.51.34 1.91-1.33 2.75-1.05 2.75-1.05.55 1.41.2 2.45.1 2.71.64.72 1.03 1.63 1.03 2.75 0 3.92-2.34 4.79-4.57 5.05.36.32.68.93.68 1.89 0 1.37-.01 2.47-.01 2.81 0 .27.18.59.69.49A10.08 10.08 0 0 0 22 12.25C22 6.59 17.52 2 12 2Z" />
      </svg>
    );
  }

  return (
    <svg className="h-4 w-4" viewBox="0 0 24 24">
      <path
        d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92a5.06 5.06 0 0 1-2.2 3.32v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.1z"
        fill="#4285F4"
      />
      <path
        d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"
        fill="#34A853"
      />
      <path
        d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z"
        fill="#FBBC05"
      />
      <path
        d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"
        fill="#EA4335"
      />
    </svg>
  );
}
