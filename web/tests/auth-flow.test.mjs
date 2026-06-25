import assert from "node:assert/strict";
import { describe, it } from "node:test";

import {
  getConsoleAuthRedirect,
  getPostLoginPath,
} from "../src/lib/auth-flow.js";

describe("auth flow routing", () => {
  it("sends users who have not accepted terms to the terms checkpoint", () => {
    assert.equal(getPostLoginPath({ terms_accepted: false }), "/accept-terms");
  });

  it("sends users with accepted terms to the console dashboard", () => {
    assert.equal(getPostLoginPath({ terms_accepted: true }), "/console/dashboard");
  });

  it("redirects unauthorized console requests to login", () => {
    assert.equal(getConsoleAuthRedirect({ status: 401 }), "/login");
  });

  it("redirects terms-required console requests to the terms checkpoint", () => {
    assert.equal(
      getConsoleAuthRedirect({ status: 403, code: "auth.terms_required" }),
      "/accept-terms"
    );
  });

  it("does not redirect non-auth console errors", () => {
    assert.equal(getConsoleAuthRedirect({ status: 500 }), null);
  });
});
