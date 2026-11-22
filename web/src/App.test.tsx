// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-2.0

import { describe, expect, it } from "vitest";
import { Health } from "./components/Shell";
import { renderToStaticMarkup } from "react-dom/server";

describe("health pill", () => {
  it("marks healthy assets", () => {
    const html = renderToStaticMarkup(<Health value="healthy" />);
    expect(html).toContain("healthy");
    expect(html).toContain("ok");
  });
});
