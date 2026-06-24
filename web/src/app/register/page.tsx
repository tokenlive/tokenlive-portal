import { Suspense } from "react";
import RegisterPageContent from "./register-page-content";

export default function RegisterPage() {
  return (
    <Suspense fallback={
      <div className="flex min-h-screen flex-col">
        <header className="sticky top-0 z-50 w-full border-b border-border/40 bg-background/80 backdrop-blur-xl">
          <div className="mx-auto flex h-14 max-w-6xl items-center justify-between px-4 sm:px-6">
            <div className="flex items-center gap-2.5">
              <span className="inline-block h-2 w-2 rounded-full bg-primary shadow-lg shadow-primary/50" />
              <span className="font-heading text-lg font-semibold tracking-tight text-foreground">
                TokenLive
              </span>
            </div>
          </div>
        </header>
        <main className="flex flex-1 items-center justify-center px-4 py-12">
          <div className="relative w-full max-w-sm">
            <div className="rounded-xl border border-border bg-card p-6 shadow-lg">
              <div className="text-center">
                <p className="text-sm text-muted-foreground">Loading...</p>
              </div>
            </div>
          </div>
        </main>
      </div>
    }>
      <RegisterPageContent />
    </Suspense>
  );
}
