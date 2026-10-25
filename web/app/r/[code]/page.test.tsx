import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, waitFor, act } from "@testing-library/react";
import { Suspense } from "react";
import ReferralPage from "./page";

const replace = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), replace }),
}));

const { recordClick } = vi.hoisted(() => ({ recordClick: vi.fn() }));

vi.mock("@/lib/api", () => ({
  affiliateApi: { recordClick },
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
    recordClick.mockReset();
    document.cookie = "aff_ref=; path=/; max-age=0";
  });

  it("sets the aff_ref cookie, records the click, and redirects home", async () => {
    recordClick.mockResolvedValue({ recorded: true });

    await renderPage("BUDI10");

    await waitFor(() => expect(replace).toHaveBeenCalledWith("/"));

    expect(document.cookie).toContain("aff_ref=BUDI10");
    expect(recordClick).toHaveBeenCalledWith("BUDI10", navigator.userAgent);
  });

  it("still redirects when the click recorder fails", async () => {
    recordClick.mockRejectedValue(new Error("rate_limited"));

    await renderPage("BUDI10");

    await waitFor(() => expect(replace).toHaveBeenCalledWith("/"));
    expect(document.cookie).toContain("aff_ref=BUDI10");
  });
});
