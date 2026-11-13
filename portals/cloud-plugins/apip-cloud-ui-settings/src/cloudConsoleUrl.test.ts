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

import { organizationsUrl, projectManageUrl } from "./cloudConsoleUrl";

describe("organizationsUrl", () => {
  it("defaults to this page's own origin, which deployments share with the Cloud console", () => {
    expect(organizationsUrl()).toBe(`${window.location.origin}/organizations`);
  });

  it("uses the given console origin, ignoring surrounding space and trailing slashes", () => {
    expect(organizationsUrl(" https://cloud-console.example// ")).toBe(
      "https://cloud-console.example/organizations",
    );
  });

  it("treats a blank console origin as unset", () => {
    expect(organizationsUrl("   ")).toBe(
      `${window.location.origin}/organizations`,
    );
  });

  it("keeps a console served under a port", () => {
    expect(organizationsUrl("http://localhost:4000")).toBe(
      "http://localhost:4000/organizations",
    );
  });

  it.each([
    "javascript:alert(1)",
    "data:text/html,hi",
    "not a url",
    "cloud-console.example",
  ])(
    "ignores %s, which is not an http(s) URL, and keeps this origin",
    (value) => {
      expect(organizationsUrl(value)).toBe(
        `${window.location.origin}/organizations`,
      );
    },
  );
});

describe("projectManageUrl", () => {
  it("names the project's manage page on this page's own origin by default", () => {
    expect(projectManageUrl("default")).toBe(
      `${window.location.origin}/projects/default/manage`,
    );
  });

  it("uses the given console origin, ignoring trailing slashes", () => {
    expect(
      projectManageUrl("orders", "https://cloud-console.example/"),
    ).toBe("https://cloud-console.example/projects/orders/manage");
  });

  it("encodes the handle as a single path segment", () => {
    expect(
      projectManageUrl("../organizations?x=1#y", "https://cloud-console.example"),
    ).toBe(
      "https://cloud-console.example/projects/..%2Forganizations%3Fx%3D1%23y/manage",
    );
  });

  it("ignores a console URL that is not http(s), and keeps this origin", () => {
    expect(projectManageUrl("default", "javascript:alert(1)")).toBe(
      `${window.location.origin}/projects/default/manage`,
    );
  });
});
