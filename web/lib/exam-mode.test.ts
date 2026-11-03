import { describe, it, expect } from "vitest";
import { t } from "@/lib/i18n";
import { examModeLabel } from "./exam-mode";

describe("examModeLabel", () => {
  const label = (mode?: string | null) => examModeLabel(mode, (key) => t("id", key));

  it.each([
    [undefined, "Standar"],
    [null, "Standar"],
    ["standard", "Standar"],
    ["utbk", "UTBK"],
    ["ielts", "IELTS"],
    ["custom", "CUSTOM"],
  ])("labels %j as %s", (mode, want) => {
    expect(label(mode)).toBe(want);
  });
});
