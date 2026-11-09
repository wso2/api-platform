/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { describe, expect, it } from "vitest";

import { resolveManageLinks, resolvePlatforms } from "./platforms";

describe("resolvePlatforms", () => {
  it("leaves out platforms with an empty URL and keeps this console", () => {
    const out = resolvePlatforms(
      { platformUrls: { "agent-manager": "https://am.example/", "api-platform": "" } },
      "https://self.example",
    );
    expect(out.map((p) => p.id)).toEqual(["agent-manager", "api-platform"]);
    expect(out[0].url).toBe("https://am.example");
    expect(out[1].url).toBe("https://self.example");
  });

  it("shows coming-soon platforms disabled", () => {
    const out = resolvePlatforms({ comingSoonPlatforms: "api-analytics, web-app-hosting" }, "/");
    expect(out.filter((p) => p.disabled).map((p) => p.id)).toEqual([
      "web-app-hosting",
      "api-analytics",
    ]);
  });

  it("drops non-http(s) URLs", () => {
    const out = resolvePlatforms(
      { platformUrls: { "identity-platform": "javascript:alert(1)" } },
      "/",
    );
    expect(out.map((p) => p.id)).toEqual(["api-platform"]);
  });
});

describe("resolveManageLinks", () => {
  it("links Cloud home and the organization settings", () => {
    expect(resolveManageLinks("https://cc.example//").map((l) => l.url)).toEqual([
      "https://cc.example",
      "https://cc.example/organizations",
    ]);
  });
  it("has none without a console origin", () => {
    expect(resolveManageLinks("")).toEqual([]);
  });
});
