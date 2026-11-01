import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { PasswordStrength, passwordStrength } from "./PasswordStrength";

describe("PasswordStrength", () => {
  it("grades by length like the student register page", () => {
    expect(passwordStrength("")).toBeNull();
    expect(passwordStrength("abc")).toBe("weak");
    expect(passwordStrength("abcdefg")).toBe("fair");
    expect(passwordStrength("abcdefghij")).toBe("strong");
  });

  it("draws nothing for an empty password and a label otherwise", () => {
    const { rerender } = render(<PasswordStrength password="" />);
    expect(screen.queryByTestId("password-strength")).toBeNull();
    rerender(<PasswordStrength password="abcdefghij" />);
    expect(screen.getByText("Kuat")).toBeInTheDocument();
  });
});
