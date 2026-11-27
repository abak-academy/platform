import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import AuthLayout from "./layout";

const replaceMock = vi.fn();
const nav = vi.hoisted(() => ({ pathname: "/login" }));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: replaceMock }),
  usePathname: () => nav.pathname,
}));

vi.mock("@/stores/auth", () => ({
  useAuthStore: (sel: (s: { token: string }) => unknown) => sel({ token: "signed-in" }),
}));

vi.mock("@/components/auth/BrandPanel", () => ({ BrandPanel: () => null }));

describe("AuthLayout with a signed-in browser", () => {
  beforeEach(() => {
    replaceMock.mockClear();
  });

  it("redirects away from /login", () => {
    nav.pathname = "/login";
    render(<AuthLayout><p>page</p></AuthLayout>);
    expect(replaceMock).toHaveBeenCalledWith("/");
    expect(screen.queryByText("page")).toBeNull();
  });

  it("still shows /reset-password so an emailed reset link works", () => {
    nav.pathname = "/reset-password";
    render(<AuthLayout><p>page</p></AuthLayout>);
    expect(replaceMock).not.toHaveBeenCalled();
    expect(screen.getByText("page")).toBeTruthy();
  });
});
