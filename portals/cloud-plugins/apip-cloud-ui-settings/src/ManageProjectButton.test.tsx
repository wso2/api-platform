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

import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import ManageProjectButton from "./ManageProjectButton";

describe("ManageProjectButton", () => {
  it("is a plain link to the project's manage page in the Cloud console", () => {
    render(
      <ManageProjectButton
        consoleUrl="https://cloud-console.example"
        projectHandle="default"
      />,
    );

    const link = screen.getByRole("link", { name: "Manage project" });
    expect(link.getAttribute("href")).toBe(
      "https://cloud-console.example/projects/default/manage",
    );
    // A full page load in the same tab: the Cloud console serves the page, not this router.
    expect(link.hasAttribute("target")).toBe(false);
  });

  it("links to this page's origin when no console origin is configured", () => {
    render(<ManageProjectButton projectHandle="default" />);

    expect(
      screen
        .getByRole("link", { name: "Manage project" })
        .getAttribute("href"),
    ).toBe(`${window.location.origin}/projects/default/manage`);
  });
});
