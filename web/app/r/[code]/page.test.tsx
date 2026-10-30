import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, waitFor, act } from "@testing-library/react";
import { Suspense } from "react";
import ReferralPage from "./page";

const replace = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), replace }),
}));

const { validateCode } = vi.hoisted(() => ({ validateCode: vi.fn() }));

vi.mock("@/lib/api", () => ({
  affiliateApi: { validateCode },
}));

async function renderPage(code: string) {
  await act(async () => {
    render(
      <Suspense fallback={null}>
        <ReferralPage params={Promise.resolve({ code })} />
      </Suspense>,
    );
  });
}

describe("ReferralPage /r/[code]", () => {
  beforeEach(() => {
    replace.mockClear();
    validateCode.mockReset();
    document.cookie = "aff_ref=; path=/; max-age=0";
  });

  it("sets the aff_ref cookie, asks the server about the code, and redirects home", async () => {
    validateCode.mockResolvedValue({ valid: true });

    await renderPage("BUDI10");

    await waitFor(() => expect(replace).toHaveBeenCalledWith("/"));

    expect(document.cookie).toContain("aff_ref=BUDI10");
    expect(validateCode).toHaveBeenCalledWith("BUDI10");
  });

  it("re-sets the cookie with the lifetime the server reports", async () => {
    validateCode.mockResolvedValue({ valid: true, cookie_days: 45 });
    const setter = vi.spyOn(document, "cookie", "set");

    await renderPage("BUDI10");
    await waitFor(() =>
      expect(setter).toHaveBeenCalledWith(expect.stringContaining(`max-age=${45 * 24 * 60 * 60}`)),
    );
    setter.mockRestore();
  });

  it("still redirects when the validate call fails", async () => {
    validateCode.mockRejectedValue(new Error("rate_limited"));

    await renderPage("BUDI10");

    await waitFor(() => expect(replace).toHaveBeenCalledWith("/"));
    expect(document.cookie).toContain("aff_ref=BUDI10");
  });
});
