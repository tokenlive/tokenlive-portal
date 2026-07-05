import assert from "node:assert/strict";
import { describe, it } from "node:test";

import {
  formatCostCNY,
  formatLatency,
  formatSuccessRate,
  formatTokens,
} from "../src/lib/usage-display.js";

describe("usage display helpers", () => {
  it("formats usage numbers", () => {
    assert.equal(formatTokens(1234567), "1,234,567");
    assert.equal(formatCostCNY("1.234567"), "¥1.234567");
    assert.equal(formatLatency(321), "321 ms");
    assert.equal(formatSuccessRate(40, 42), "95.2%");
  });

  it("formats empty usage values", () => {
    assert.equal(formatCostCNY(""), "-");
    assert.equal(formatLatency(0), "-");
    assert.equal(formatSuccessRate(0, 0), "-");
  });
});
