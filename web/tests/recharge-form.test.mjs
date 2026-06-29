import assert from "node:assert/strict";
import { describe, it } from "node:test";

import {
  buildCreateRechargeRequest,
  formatLedgerDirection,
  formatLedgerType,
  formatRechargeStatus,
} from "../src/lib/recharge-form.js";

describe("recharge form helpers", () => {
  it("builds a trimmed recharge request", () => {
    assert.deepEqual(
      buildCreateRechargeRequest({
        amountCNY: "20.50",
        paymentMethod: " bank_transfer ",
        contact: " ops@example.com ",
        note: " invoice needed ",
      }),
      {
        amount_micro_cny: 20_500_000,
        payment_method: "bank_transfer",
        contact: "ops@example.com",
        note: "invoice needed",
      }
    );
  });

  it("keeps empty amount invalid for the backend validator", () => {
    assert.equal(
      buildCreateRechargeRequest({
        amountCNY: "",
        paymentMethod: "bank_transfer",
        contact: "ops@example.com",
        note: "",
      }).amount_micro_cny,
      0
    );
  });

  it("formats known and unknown statuses", () => {
    assert.equal(formatRechargeStatus("pending"), "Pending");
    assert.equal(formatRechargeStatus("approved"), "Approved");
    assert.equal(formatRechargeStatus("missing"), "Unknown");
  });

  it("formats ledger entry types and directions", () => {
    assert.equal(formatLedgerType("trial_grant"), "Trial credit");
    assert.equal(formatLedgerType("consumption"), "Usage");
    assert.equal(formatLedgerType("unknown"), "Unknown");
    assert.equal(formatLedgerDirection("credit"), "+");
    assert.equal(formatLedgerDirection("debit"), "-");
  });
});
