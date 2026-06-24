"use client";

import { useState } from "react";
import type { ModelListItem } from "@/types/api";
import { ModelCard } from "./model-card";

interface ModelCatalogProps {
  models: ModelListItem[] | null;
}

export function ModelCatalog({ models }: ModelCatalogProps) {
  const [query, setQuery] = useState("");

  const filtered = (models || []).filter((m) => {
    if (!query) return true;
    const q = query.toLowerCase();
    return (
      m.display_name.toLowerCase().includes(q) ||
      m.model_id.toLowerCase().includes(q) ||
      m.short_description?.toLowerCase().includes(q)
    );
  });

  return (
    <section className="mx-auto max-w-6xl px-4 py-10 sm:px-6">
      {/* Search + count */}
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h2 className="font-heading text-lg font-semibold text-foreground">
            Available Models
          </h2>
          <p className="mt-0.5 text-sm text-muted-foreground">
            All models accessible through a single{" "}
            <code className="rounded bg-muted px-1.5 py-0.5 font-mono text-xs">
              POST /v1/chat/completions
            </code>{" "}
            endpoint
          </p>
        </div>
        <div className="relative w-full sm:w-64">
          <input
            type="text"
            placeholder="Search models..."
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            className="h-9 w-full rounded-md border border-input bg-background px-3 pl-8 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-primary/40"
          />
          <svg
            className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground"
            xmlns="http://www.w3.org/2000/svg"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinecap="round"
            strokeLinejoin="round"
          >
            <circle cx="11" cy="11" r="8" />
            <path d="m21 21-4.3-4.3" />
          </svg>
        </div>
      </div>

      {/* Grid */}
      {filtered.length > 0 ? (
        <div className="mt-6 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {filtered.map((model) => (
            <ModelCard key={model.model_id} model={model} />
          ))}
        </div>
      ) : models === null ? (
        <div className="mt-12 flex flex-col items-center gap-3 text-center">
          <span className="tl-node tl-node--lg" />
          <p className="text-sm text-muted-foreground">
            Cannot reach API. Start the backend server to browse models.
          </p>
        </div>
      ) : (
        <div className="mt-12 flex flex-col items-center gap-3 text-center">
          <p className="text-sm text-muted-foreground">
            No models found matching &ldquo;{query}&rdquo;
          </p>
        </div>
      )}
    </section>
  );
}
