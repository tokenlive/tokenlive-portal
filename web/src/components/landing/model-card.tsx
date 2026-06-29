"use client";

import Image from "next/image";
import Link from "next/link";
import { ModelListItem } from "@/types/api";
import { buildModelDetailHref, formatModelPrice } from "@/lib/model-display";

interface ModelCardProps {
  model: ModelListItem;
}

const MODALITY_BADGE: Record<string, string> = {
  text: "Text",
  image: "Vision",
  audio: "Audio",
  video: "Video",
};

export function ModelCard({ model }: ModelCardProps) {
  const displayName = model.display_name || model.slug;
  const href = buildModelDetailHref(model);
  const inputModalities = (model.input_modalities || [])
    .map((m) => MODALITY_BADGE[m] || m)
    .join(" · ");
  const outputModalities = (model.output_modalities || [])
    .map((m) => MODALITY_BADGE[m] || m)
    .join(" · ");

  return (
    <Link
      href={href}
      className="group relative flex min-h-48 flex-col rounded-lg border border-border bg-card p-5 transition-all hover:border-primary/40 hover:bg-card/90 hover:shadow-[0_0_0_1px_oklch(0.623_0.214_259.815_/_18%)] tl-focus-ring"
    >
      {/* Header: Logo + Name */}
      <div className="flex items-start justify-between gap-3">
        <div className="flex items-center gap-3">
          {model.logo_url ? (
            <Image
              src={model.logo_url}
              alt={displayName}
              width={36}
              height={36}
              className="h-9 w-9 rounded-md object-contain"
              unoptimized
            />
          ) : (
            <div className="flex h-9 w-9 items-center justify-center rounded-md bg-muted font-heading text-sm font-semibold text-muted-foreground">
              {displayName.charAt(0).toUpperCase()}
            </div>
          )}
          <div>
            <h3 className="font-heading text-sm font-semibold leading-tight text-foreground">
              {displayName}
            </h3>
            <p className="mt-0.5 font-mono text-[11px] text-muted-foreground">
              {model.model_id}
            </p>
          </div>
        </div>
        <span className="tl-node tl-node--sm mt-2 opacity-0 transition-opacity group-hover:opacity-100" />
      </div>

      {/* Description */}
      {model.short_description && (
        <p className="mt-3 line-clamp-2 text-sm leading-relaxed text-muted-foreground">
          {model.short_description}
        </p>
      )}

      {/* Modalities */}
      {(inputModalities || outputModalities) && (
        <div className="mt-3 flex flex-wrap gap-1.5">
          {inputModalities && (
            <span className="inline-flex items-center gap-1 rounded-md bg-secondary px-2 py-0.5 text-[11px] font-medium text-secondary-foreground">
              <span className="text-muted-foreground">IN</span> {inputModalities}
            </span>
          )}
          {outputModalities && (
            <span className="inline-flex items-center gap-1 rounded-md bg-secondary px-2 py-0.5 text-[11px] font-medium text-secondary-foreground">
              <span className="text-muted-foreground">OUT</span> {outputModalities}
            </span>
          )}
        </div>
      )}

      {/* Metrics + Price */}
      <div className="mt-auto pt-4">
        <div className="flex items-center justify-between text-[11px]">
          {/* Latency */}
          <div className="flex items-center gap-3">
            {model.metrics?.ttft_p50_ms !== null && model.metrics?.ttft_p50_ms !== undefined && (
              <span className="font-mono text-muted-foreground">
                <span className="text-foreground">{model.metrics.ttft_p50_ms}</span>ms p50
              </span>
            )}
            {model.context_length !== null && model.context_length !== undefined && (
              <span className="font-mono text-muted-foreground">
                <span className="text-foreground">
                  {(model.context_length / 1000).toFixed(0)}k
                </span>{" "}
                ctx
              </span>
            )}
          </div>

          {/* Pricing */}
          <div className="flex items-center gap-2 font-mono">
            <span className="text-muted-foreground">
              <span className="text-foreground">
                {formatModelPrice(model.price?.input_price)}
              </span>{" "}
              in
            </span>
            <span className="text-muted-foreground">
              <span className="text-foreground">
                {formatModelPrice(model.price?.output_price)}
              </span>{" "}
              out
            </span>
          </div>
        </div>
      </div>
    </Link>
  );
}
