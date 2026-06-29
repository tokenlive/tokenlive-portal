import { notFound } from "next/navigation";
import Link from "next/link";
import Image from "next/image";
import { fetchModelDetail } from "@/lib/api";
import { Header } from "@/components/layout/header";
import {
  formatModelPrice,
  formatPercentMetric,
  formatThroughput,
} from "@/lib/model-display";
import type { Metadata } from "next";

type Props = {
  params: Promise<{ slug: string }>;
};

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { slug } = await params;
  let title = "Model Details";
  let description = "View model details on TokenLive";

  try {
    const model = await fetchModelDetail(slug);
    title = model.seo_title || `${model.display_name} | TokenLive`;
    description =
      model.seo_description ||
      model.long_description ||
      model.short_description ||
      description;
  } catch {
    // Fallback to default metadata
  }

  return { title, description };
}

export default async function ModelDetailPage({ params }: Props) {
  const { slug } = await params;

  let model = null;
  try {
    model = await fetchModelDetail(slug);
  } catch {
    // API unreachable
  }

  if (!model) {
    notFound();
  }

  return (
    <>
      <Header />
      <main className="flex-1">
        <div className="mx-auto max-w-4xl px-4 py-12 sm:px-6">
          {/* Breadcrumb */}
          <nav className="mb-6 flex items-center gap-2 text-sm text-muted-foreground">
            <Link href="/" className="hover:text-foreground transition-colors">
              Models
            </Link>
            <span>/</span>
            <span className="text-foreground">{model.display_name}</span>
          </nav>

          {/* Header */}
          <div className="mb-8 grid gap-6 lg:grid-cols-[1fr_260px]">
            <div className="flex items-start gap-4">
              {model.logo_url ? (
                <Image
                  src={model.logo_url}
                  alt={model.display_name}
                  width={64}
                  height={64}
                  className="h-16 w-16 rounded-lg object-contain"
                  unoptimized
                />
              ) : (
                <div className="flex h-16 w-16 items-center justify-center rounded-lg bg-muted font-heading text-2xl font-semibold text-muted-foreground">
                  {model.display_name.charAt(0).toUpperCase()}
                </div>
              )}
              <div>
                <div className="flex items-center gap-2">
                  <h1 className="font-heading text-2xl font-semibold text-foreground">
                    {model.display_name}
                  </h1>
                  {model.featured && (
                    <span className="tl-node tl-node--sm tl-node--active" />
                  )}
                </div>
                <p className="mt-1 font-mono text-sm text-muted-foreground">
                  {model.model_id}
                </p>
                <p className="mt-2 text-sm text-muted-foreground">
                  {model.short_description}
                </p>
              </div>
            </div>
            <div className="rounded-lg border border-primary/20 bg-primary/5 p-4">
              <p className="font-heading text-sm font-medium text-primary">
                Ready on the unified endpoint
              </p>
              <p className="mt-2 text-sm leading-relaxed text-muted-foreground">
                Use this exact model id with your TokenLive key. No provider
                routing details are exposed in the portal.
              </p>
              <Link
                href="/console/api-keys"
                className="mt-4 inline-flex rounded-md bg-primary px-3 py-2 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90 tl-focus-ring"
              >
                Create API key
              </Link>
            </div>
          </div>

          {/* Key info grid */}
          <div className="mb-8 grid grid-cols-2 gap-4 sm:grid-cols-4">
            {model.context_length && (
              <InfoCard
                label="Context Length"
                value={`${(model.context_length / 1000).toFixed(0)}k`}
              />
            )}
            {model.price && (
              <>
                <InfoCard
                  label="Input Price"
                  value={formatModelPrice(model.price.input_price)}
                />
                <InfoCard
                  label="Output Price"
                  value={formatModelPrice(model.price.output_price)}
                />
                {model.price.cached_price != null && (
                  <InfoCard
                    label="Cache Hit Price"
                    value={formatModelPrice(model.price.cached_price)}
                  />
                )}
                {model.price.cache_creation_price != null && (
                  <InfoCard
                    label="Cache Creation Price"
                    value={formatModelPrice(model.price.cache_creation_price)}
                  />
                )}
              </>
            )}
            {model.metrics && model.metrics.ttft_p50_ms && (
              <InfoCard
                label="Avg TTFT"
                value={`${model.metrics.ttft_p50_ms}ms`}
              />
            )}
          </div>

          {model.metrics && (
            <div className="mb-8">
              <h2 className="mb-3 font-heading text-sm font-medium text-muted-foreground uppercase tracking-wider">
                Recent Performance
              </h2>
              <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
                <InfoCard
                  label="Availability"
                  value={formatPercentMetric(model.metrics.availability)}
                />
                <InfoCard
                  label="Success Rate"
                  value={formatPercentMetric(model.metrics.success_rate)}
                />
                <InfoCard
                  label="P95 TTFT"
                  value={
                    model.metrics.ttft_p95_ms
                      ? `${model.metrics.ttft_p95_ms}ms`
                      : "-"
                  }
                />
                <InfoCard
                  label="Speed"
                  value={formatThroughput(model.metrics.response_speed)}
                />
              </div>
              <p className="mt-2 text-xs text-muted-foreground">
                Window: {model.metrics.window || "recent"} · Samples:{" "}
                {model.metrics.sample_count}
              </p>
            </div>
          )}

          {/* Modalities */}
          {((model.input_modalities?.length ?? 0) > 0 || (model.output_modalities?.length ?? 0) > 0) && (
            <div className="mb-8">
              <h2 className="mb-3 font-heading text-sm font-medium text-muted-foreground uppercase tracking-wider">
                Modalities
              </h2>
              <div className="flex flex-wrap gap-2">
                {model.input_modalities?.map((modality) => (
                  <span
                    key={`in-${modality}`}
                    className="inline-flex items-center gap-1 rounded-md bg-secondary px-3 py-1 text-xs font-medium text-secondary-foreground"
                  >
                    <span className="text-muted-foreground">IN</span>
                    {modality.charAt(0).toUpperCase() + modality.slice(1)}
                  </span>
                ))}
                {model.output_modalities?.map((modality) => (
                  <span
                    key={`out-${modality}`}
                    className="inline-flex items-center gap-1 rounded-md bg-secondary px-3 py-1 text-xs font-medium text-secondary-foreground"
                  >
                    <span className="text-muted-foreground">OUT</span>
                    {modality.charAt(0).toUpperCase() + modality.slice(1)}
                  </span>
                ))}
              </div>
            </div>
          )}

          {/* Long description */}
          {model.long_description && (
            <div className="mb-8">
              <h2 className="mb-3 font-heading text-sm font-medium text-muted-foreground uppercase tracking-wider">
                Description
              </h2>
              <div className="rounded-lg border border-border bg-card p-5">
                <p className="text-sm text-foreground leading-relaxed whitespace-pre-wrap">
                  {model.long_description}
                </p>
              </div>
            </div>
          )}

          {/* API Example */}
          <div id="quick-start">
            <h2 className="mb-3 font-heading text-sm font-medium text-muted-foreground uppercase tracking-wider">
              First Call
            </h2>
            <div className="rounded-lg border border-border bg-card p-5">
              <p className="mb-3 text-sm text-muted-foreground">
                Copy this after creating a key. The model id below is the
                public TokenLive id for this page.
              </p>
              <pre className="overflow-x-auto rounded-md bg-background p-4 font-mono text-sm leading-relaxed text-foreground">
                <code>{`curl -X POST https://api.tokenlive.ai/v1/chat/completions \\
  -H "Authorization: Bearer tl_live_YOUR_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "${model.model_id}",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'`}</code>
              </pre>
            </div>
          </div>
        </div>
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

function InfoCard({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border border-border bg-card p-4">
      <p className="text-xs font-medium text-muted-foreground uppercase tracking-wider">
        {label}
      </p>
      <p className="mt-1 font-heading text-lg font-semibold text-foreground">
        {value}
      </p>
    </div>
  );
}
