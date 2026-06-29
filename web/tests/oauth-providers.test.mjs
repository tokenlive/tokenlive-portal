import assert from "node:assert/strict";
import { describe, it } from "node:test";

import {
  getOAuthProviderRows,
  OAUTH_PROVIDERS,
} from "../src/lib/oauth-providers.js";

describe("oauth provider display contract", () => {
  it("keeps Google enabled with login and bind URLs", () => {
    const google = OAUTH_PROVIDERS.find((p) => p.id === "google");

    assert.deepEqual(
      {
        label: google?.label,
        loginHref: google?.loginHref,
        bindHref: google?.bindHref,
        enabled: google?.enabled,
      },
      {
        label: "Google",
        loginHref: "/api/auth/google/login",
        bindHref: "/api/auth/google/bind",
        enabled: true,
      }
    );
  });

  it("keeps GitHub enabled with login and bind URLs", () => {
    const github = OAUTH_PROVIDERS.find((p) => p.id === "github");

    assert.deepEqual(
      {
        label: github?.label,
        loginHref: github?.loginHref,
        bindHref: github?.bindHref,
        enabled: github?.enabled,
        unavailableLabel: github?.unavailableLabel,
      },
      {
        label: "GitHub",
        loginHref: "/api/auth/github/login",
        bindHref: "/api/auth/github/bind",
        enabled: true,
        unavailableLabel: null,
      }
    );
  });

  it("marks connected providers and leaves unconnected providers visible", () => {
    const rows = getOAuthProviderRows([
      {
        provider: "google",
        display_name: "Ada",
        email: "ada@example.com",
      },
    ]);

    assert.deepEqual(
      rows.map((row) => ({
        id: row.id,
        connected: row.connected,
        disabled: row.disabled,
      })),
      [
        { id: "google", connected: true, disabled: false },
        { id: "github", connected: false, disabled: false },
      ]
    );
  });
});
