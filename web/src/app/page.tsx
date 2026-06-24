import { fetchModels } from "@/lib/api";
import { Header } from "@/components/layout/header";
import { ModelCatalog } from "@/components/landing/model-catalog";
import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Models",
  description:
    "Browse all available AI models on TokenLive — unified API gateway for OpenAI, Anthropic, Google, DeepSeek, Qwen, and more.",
};

export const dynamic = "force-dynamic";

export default async function HomePage() {
  let models = null;
  try {
    const res = await fetchModels();
    models = res.data;
  } catch {
    // API not reachable during build; show empty state
  }

  return (
    <>
      <Header />
      <main className="flex-1">
        {/* Hero */}
        <section className="relative overflow-hidden border-b border-border/40">
          {/* Ambient glow — the signature element */}
          <div className="pointer-events-none absolute inset-0 flex items-center justify-center">
            <div className="h-72 w-[480px] rounded-full bg-primary/5 blur-3xl" />
          </div>

          <div className="relative mx-auto max-w-6xl px-4 py-16 sm:px-6 sm:py-20">
            <div className="max-w-2xl">
              <div className="mb-4 flex items-center gap-2">
                <span className="tl-node tl-node--active" />
                <span className="font-mono text-xs uppercase tracking-wider text-muted-foreground">
                  AI Model Gateway
                </span>
              </div>

              <h1 className="font-heading text-3xl font-bold leading-tight tracking-tight sm:text-4xl lg:text-5xl">
                One API to reach
                <br />
                <span className="text-primary">every model</span>
              </h1>

              <p className="mt-4 max-w-lg text-sm leading-relaxed text-muted-foreground sm:text-lg">
                Route requests to OpenAI, Anthropic, Google, DeepSeek, Qwen and
                more through a single unified endpoint. Monitor usage, manage
                access keys, and control spending from one console.
              </p>

              {/* Stats */}
              <div className="mt-8 flex items-center gap-6 text-sm">
                <div className="flex items-center gap-2">
                  <span className="font-heading text-2xl font-bold text-foreground">
                    {models?.length || "—"}
                  </span>
                  <span className="text-muted-foreground">models</span>
                </div>
                <span className="h-5 w-px bg-border" />
                <div className="flex items-center gap-2">
                  <span className="font-heading text-2xl font-bold text-foreground">
                    6
                  </span>
                  <span className="text-muted-foreground">providers</span>
                </div>
                <span className="h-5 w-px bg-border" />
                <div className="flex items-center gap-2">
                  <span className="font-heading text-2xl font-bold text-foreground">
                    99.9%
                  </span>
                  <span className="text-muted-foreground">uptime</span>
                </div>
              </div>
            </div>
          </div>
        </section>

        {/* Model catalog */}
        <ModelCatalog models={models} />
      </main>

      {/* Footer */}
      <footer className="border-t border-border/40 py-8">
        <div className="mx-auto max-w-6xl px-4 sm:px-6">
          <div className="flex flex-col items-center justify-between gap-4 sm:flex-row">
            <div className="flex items-center gap-2">
              <span className="tl-node tl-node--sm" />
              <span className="font-heading text-sm font-medium text-muted-foreground">
                TokenLive
              </span>
            </div>
            <p className="text-xs text-muted-foreground">
              Unified LLM API Gateway
            </p>
          </div>
        </div>
      </footer>
    </>
  );
}
