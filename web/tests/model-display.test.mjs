import assert from "node:assert/strict";
import { describe, it } from "node:test";

import {
  buildModelDetailHref,
  formatModelPrice,
  formatPercentMetric,
  formatThroughput,
} from "../src/lib/model-display.js";

describe("model display helpers", () => {
  it("builds detail links from model slugs", () => {
    assert.equal(
      buildModelDetailHref({ slug: "openai-gpt-4o", model_id: "gpt-4o" }),
      "/models/openai-gpt-4o"
    );
  });

  it("falls back to encoded model ids when a slug is missing", () => {
    assert.equal(
      buildModelDetailHref({ model_id: "openai/gpt-4o mini" }),
      "/models/openai%2Fgpt-4o%20mini"
    );
  });

  it("formats missing prices as unavailable", () => {
    assert.equal(formatModelPrice(undefined), "-");
  });

  it("formats price values as RMB per million tokens", () => {
    assert.equal(formatModelPrice(2), "¥2.00/1M");
  });

  it("formats fractional metrics as percentages", () => {
    assert.equal(formatPercentMetric(0.997), "99.7%");
  });

  it("formats response speed with token units", () => {
    assert.equal(formatThroughput(83.45), "83.5 tok/s");
  });
});
