import assert from "node:assert/strict";
import { describe, it } from "node:test";

import {
  getActivationStepAction,
  getNextActivationAction,
} from "../src/lib/activation-actions.js";

describe("activation action helpers", () => {
  it("routes pending API key creation to the API keys page", () => {
    assert.deepEqual(
      getActivationStepAction({ key: "api_key", status: "pending" }),
      {
        href: "/console/api-keys",
        label: "Create key",
      }
    );
  });

  it("routes first API calls to the quick start section", () => {
    assert.deepEqual(
      getActivationStepAction({ key: "first_call", status: "pending" }),
      {
        href: "/console/dashboard#quick-start",
        label: "View curl",
      }
    );
  });

  it("routes runtime activation to the dashboard waiting state", () => {
    assert.deepEqual(
      getActivationStepAction({
        key: "runtime_activation",
        status: "pending",
      }),
      {
        href: "/console/dashboard",
        label: "Waiting for admin",
      }
    );
  });

  it("does not show actions for completed steps", () => {
    assert.equal(
      getActivationStepAction({ key: "api_key", status: "completed" }),
      null
    );
  });

  it("selects the first pending activation action", () => {
    const steps = [
      { key: "trial_credit", status: "completed" },
      { key: "api_key", status: "pending" },
      { key: "first_call", status: "pending" },
    ];

    assert.deepEqual(getNextActivationAction(steps), {
      href: "/console/api-keys",
      label: "Create key",
    });
  });
});
