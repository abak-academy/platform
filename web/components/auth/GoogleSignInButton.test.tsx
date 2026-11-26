import { cleanup, fireEvent, render, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { GoogleSignInButton } from "./GoogleSignInButton";

const mutate = vi.fn();
const replace = vi.fn();

vi.mock("@/lib/hooks/auth", () => ({
  useGoogleLogin: () => ({ mutate, isPending: false }),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace }),
}));

describe("GoogleSignInButton", () => {
  beforeEach(() => {
    vi.stubEnv("NEXT_PUBLIC_GOOGLE_CLIENT_ID", "test-client");
    mutate.mockReset();
    replace.mockReset();
    delete window.google;
    document.querySelectorAll('script[src*="gsi/client"]').forEach((script) => script.remove());
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllEnvs();
  });

  it("posts the GSI credential as an id_token input", async () => {
    let callback: ((response: { credential: string }) => void) | undefined;
    window.google = {
      accounts: {
        id: {
          initialize: vi.fn((config) => {
            callback = config.callback;
          }),
          renderButton: vi.fn(),
        },
      },
    };

    render(<GoogleSignInButton />);
    await waitFor(() => expect(callback).toBeDefined());

    callback?.({ credential: "google-id-token" });

    expect(mutate).toHaveBeenCalledWith(
      { id_token: "google-id-token" },
      expect.objectContaining({ onSuccess: expect.any(Function) }),
    );
  });

  it("waits for an existing in-flight GSI script before rendering", async () => {
    const first = render(<GoogleSignInButton />);
    const script = document.querySelector('script[src*="gsi/client"]');
    expect(script).not.toBeNull();
    first.unmount();

    const initialize = vi.fn();
    const renderButton = vi.fn();
    render(<GoogleSignInButton />);
    window.google = { accounts: { id: { initialize, renderButton } } };
    fireEvent.load(script!);

    await waitFor(() => expect(initialize).toHaveBeenCalledTimes(1));
    expect(renderButton).toHaveBeenCalledTimes(1);
  });

  it("routes an incomplete Google student to profile completion after login", async () => {
    let callback: ((response: { credential: string }) => void) | undefined;
    window.google = {
      accounts: {
        id: {
          initialize: vi.fn((config) => {
            callback = config.callback;
          }),
          renderButton: vi.fn(),
        },
      },
    };
    render(<GoogleSignInButton />);
    await waitFor(() => expect(callback).toBeDefined());
    callback?.({ credential: "google-id-token" });

    const options = mutate.mock.calls[0][1];
    options.onSuccess({
      user: { role: "student", auth_provider: "google" },
    });

    expect(replace).toHaveBeenCalledWith("/complete-profile");
  });

  it.each([
    ["/cart", "student", "/cart"],
    ["//evil.example", "student", "/"],
    ["/cart", "super_admin", "/admin"],
  ])("next=%s as %s lands on %s", async (next, role, landing) => {
    let callback: ((response: { credential: string }) => void) | undefined;
    window.google = {
      accounts: {
        id: {
          initialize: vi.fn((config) => {
            callback = config.callback;
          }),
          renderButton: vi.fn(),
        },
      },
    };
    render(<GoogleSignInButton next={next} />);
    await waitFor(() => expect(callback).toBeDefined());
    callback?.({ credential: "google-id-token" });

    mutate.mock.calls[0][1].onSuccess({ user: { role, auth_provider: "google", school_id: 1, grade: 10 } });

    expect(replace).toHaveBeenCalledWith(landing);
  });

  it("renders nothing when the Google client id is empty", () => {
    vi.stubEnv("NEXT_PUBLIC_GOOGLE_CLIENT_ID", "");
    const initialize = vi.fn();
    window.google = {
      accounts: { id: { initialize, renderButton: vi.fn() } },
    };

    const { container } = render(<GoogleSignInButton />);

    expect(container).toBeEmptyDOMElement();
    expect(initialize).not.toHaveBeenCalled();
  });
});
