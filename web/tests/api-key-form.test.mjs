import assert from "node:assert/strict";
import { describe, it } from "node:test";

import {
  buildCreateAPIKeyRequest,
  formatAPIKeyLimit,
} from "../src/lib/api-key-form.js";

describe("api key form helpers", () => {
  it("builds a minimal create request with a trimmed name", () => {
    assert.deepEqual(
      buildCreateAPIKeyRequest({
        name: "  production  ",
        dailyLimitCNY: "",
        monthlyLimitCNY: "",
        expiresOn: "",
      }),
      { name: "production" }
    );
  });

  it("converts yuan inputs to micro-cny limits", () => {
    assert.deepEqual(
      buildCreateAPIKeyRequest({
        name: "prod",
        dailyLimitCNY: "12.34",
        monthlyLimitCNY: "100",
        expiresOn: "",
      }),
      {
        name: "prod",
        daily_limit_micro_cny: 12_340_000,
        monthly_limit_micro_cny: 100_000_000,
      }
    );
  });

  it("converts an expiration date to the end of that UTC day", () => {
    assert.deepEqual(
      buildCreateAPIKeyRequest({
        name: "prod",
        dailyLimitCNY: "",
        monthlyLimitCNY: "",
        expiresOn: "2026-07-01",
      }),
      {
        name: "prod",
        expires_at: "2026-07-01T23:59:59.000Z",
      }
    );
  });

  it("formats missing limits as unlimited", () => {
    assert.equal(formatAPIKeyLimit(), "Unlimited");
  });

  it("formats micro-cny limits as yuan", () => {
    assert.equal(formatAPIKeyLimit(1_230_000), "¥1.23");
  });
});
