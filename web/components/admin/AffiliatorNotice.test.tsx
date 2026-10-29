import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, renderHook, screen } from "@testing-library/react";
import { AffiliatorNotice, useAffiliatorReadOnly } from "./AffiliatorNotice";

let me: { data?: { status: string } };
vi.mock("@/lib/hooks/affiliates", () => ({ useAffiliatorMe: () => me }));

describe("AffiliatorNotice", () => {
  beforeEach(() => {
    me = { data: { status: "active" } };
  });

  it("says nothing for an active affiliator", () => {
    render(<AffiliatorNotice />);

    expect(screen.queryByRole("status")).not.toBeInTheDocument();
  });

  it("tells a deactivated affiliator the account is read-only", () => {
    me = { data: { status: "deactivated" } };
    render(<AffiliatorNotice />);

    expect(screen.getByRole("status")).toHaveTextContent("Akun Anda dinonaktifkan");
    expect(screen.getByRole("status").className).toContain("bg-warn-bg");
  });

  it("does not guess before the profile has loaded", () => {
    me = {};
    const { result } = renderHook(() => useAffiliatorReadOnly());

    expect(result.current).toBe(false);
  });
});
