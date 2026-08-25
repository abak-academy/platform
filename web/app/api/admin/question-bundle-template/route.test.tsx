import { describe, expect, it } from "vitest";
import { POST } from "./route";

describe("question bundle template serializer", () => {
  it("returns self-contained FE-authored fragments with the closed token contract", async () => {
    const response = await POST();
    expect(response.status).toBe(200);
    const body = await response.json();

    expect(body.template.document).toContain("{{bundle_title}}");
    expect(body.template.document).toContain("{{tests_html}}");
    expect(body.template.test).toContain("{{questions_html}}");
    expect(body.template.question).toContain("{{answer_html}}");
    expect(body.template.option).toContain("{{option_text}}");
    expect(body.template.answer).toContain("KUNCI JAWABAN");
    expect(body.template.answer_item).toContain("{{answer_item}}");

    for (const fragment of Object.values(body.template) as string[]) {
      expect(fragment).not.toMatch(/https?:\/\//);
      expect(fragment).not.toContain("<script");
    }
  });
});
