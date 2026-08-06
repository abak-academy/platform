import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";

let VideoPlayer: (typeof import("./VideoPlayer"))["VideoPlayer"];

// Real YT.Player REPLACES the element it's given with its own <iframe>
// instead of mounting inside it, and destroy() does not restore the
// original node. This fake reproduces that behavior so tests actually
// exercise the replaced/detached-node hazard instead of a happy-path stand-in.
class FakePlayer {
  static instances: FakePlayer[] = [];
  destroyed = false;
  wasConnectedAtConstruction: boolean;
  private iframeEl: HTMLIFrameElement;

  constructor(
    el: HTMLElement,
    public opts: {
      width?: string | number;
      height?: string | number;
      events?: { onReady?: () => void; onError?: () => void };
    },
  ) {
    this.wasConnectedAtConstruction = el.isConnected;
    FakePlayer.instances.push(this);
    this.iframeEl = document.createElement("iframe");
    el.replaceWith(this.iframeEl);
    opts.events?.onReady?.();
  }

  playVideo() {}
  pauseVideo() {}
  seekTo() {}
  getCurrentTime() {
    return 0;
  }
  getDuration() {
    return 100;
  }
  getVideoLoadedFraction() {
    return 0;
  }
  mute() {}
  unMute() {}
  destroy() {
    this.destroyed = true;
    this.iframeEl.remove();
  }
  /** Simulates a real YT.Player error event (private/deleted video, region block, etc). */
  triggerError() {
    this.opts.events?.onError?.();
  }
}

describe("VideoPlayer", () => {
  beforeEach(async () => {
    // loadYoutubeApi() caches a module-level promise across every mount on
    // the page. Reset the module before each test so that cache — and the
    // retry-after-failure fix for it — behaves deterministically regardless
    // of test order, instead of one test's script tag leaking into another's.
    vi.resetModules();
    vi.stubGlobal("YT", undefined);
    FakePlayer.instances = [];
    ({ VideoPlayer } = await import("./VideoPlayer"));
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it("renders the empty state when there is no video", () => {
    render(<VideoPlayer title="Pelajaran 1" />);
    expect(screen.getByText(/Video belum tersedia/i)).toBeInTheDocument();
  });

  it("renders a click shield over the player", () => {
    const { container } = render(
      <VideoPlayer videoRef="https://www.youtube.com/watch?v=abc123" title="L1" />,
    );
    expect(container.querySelector('[data-testid="video-shield"]')).not.toBeNull();
  });

  it("exposes no link to youtube.com anywhere in the tree", () => {
    const { container } = render(
      <VideoPlayer videoRef="https://www.youtube.com/watch?v=abc123" title="L1" />,
    );
    const hrefs = Array.from(container.querySelectorAll("a")).map((a) => a.getAttribute("href") ?? "");
    expect(hrefs.some((h) => h.includes("youtube.com") || h.includes("youtu.be"))).toBe(false);
  });

  it("renders our own control bar, not YouTube's", () => {
    render(<VideoPlayer videoRef="abc123" title="L1" />);
    expect(screen.getByRole("button", { name: /putar|play/i })).toBeInTheDocument();
    expect(screen.getByRole("slider", { name: /posisi|progress/i })).toBeInTheDocument();
  });

  // This exercises the forceFallback TEST SEAM only — it does not simulate a
  // load failure. The real failure path is covered by "recovers on a later
  // mount after the YouTube API script fails to load once" below.
  it("renders the shield-less plain embed when the forceFallback seam is set", async () => {
    const { container } = render(
      <VideoPlayer videoRef="abc123" title="L1" forceFallback />,
    );
    const iframe = container.querySelector("iframe");
    expect(iframe).not.toBeNull();
    expect(container.querySelector('[data-testid="video-shield"]')).toBeNull();
  });

  it("constructs the player with fill-the-wrapper sizing options", async () => {
    vi.stubGlobal("YT", { Player: FakePlayer });

    render(<VideoPlayer videoRef="abc123" title="L1" />);

    await waitFor(() => expect(FakePlayer.instances).toHaveLength(1));
    // Without these the IFrame API stamps its 640x390 default on the iframe.
    expect(FakePlayer.instances[0].opts.width).toBe("100%");
    expect(FakePlayer.instances[0].opts.height).toBe("100%");
  });

  it("gives the mount host a fill-the-wrapper style contract for the injected iframe", () => {
    // jsdom has no layout, so this pins the CSS contract rather than the pixels:
    // the API writes width/height ATTRIBUTES, which author CSS outranks.
    const { container } = render(<VideoPlayer videoRef="abc123" title="L1" />);
    const mount = container.querySelector('[data-testid="video-mount"]');
    expect(mount).not.toBeNull();
    expect(mount!.className).toContain("size-full");
    expect(mount!.className).toContain("[&>iframe]:size-full");
  });

  it("shows an in-app error card with no YouTube iframe when a video errors, and recovers on the next lesson", async () => {
    vi.stubGlobal("YT", { Player: FakePlayer });

    const { rerender, container } = render(
      <VideoPlayer videoRef="videoBlocked" title="Pelajaran 1" />,
    );

    await waitFor(() => expect(FakePlayer.instances).toHaveLength(1));
    FakePlayer.instances[0].triggerError();

    await waitFor(() => {
      expect(screen.getByText(/Video tidak dapat diputar/i)).toBeInTheDocument();
    });

    // The shield-less fallback embed would put YouTube's own error screen —
    // and its live "Watch on YouTube" link — back on the page.
    const srcs = Array.from(container.querySelectorAll("iframe")).map(
      (f) => f.getAttribute("src") ?? "",
    );
    expect(srcs.some((s) => s.includes("youtube-nocookie.com") || s.includes("youtube.com"))).toBe(
      false,
    );

    rerender(<VideoPlayer videoRef="videoGood" title="Pelajaran 2" />);

    await waitFor(() => expect(FakePlayer.instances).toHaveLength(2));
    expect(container.querySelector('[data-testid="video-shield"]')).not.toBeNull();
    expect(screen.queryByText(/Video tidak dapat diputar/i)).toBeNull();
  });

  it("rebuilds the player against a fresh, still-attached node when videoRef changes on a live instance", async () => {
    // page.tsx renders <VideoPlayer> with no `key`, so picking a different
    // lesson changes videoRef on this same instance rather than remounting.
    vi.stubGlobal("YT", { Player: FakePlayer });

    const { rerender, container } = render(
      <VideoPlayer videoRef="videoA" title="Pelajaran 1" />,
    );

    await waitFor(() => expect(FakePlayer.instances).toHaveLength(1));
    const first = FakePlayer.instances[0];
    expect(first.wasConnectedAtConstruction).toBe(true);

    rerender(<VideoPlayer videoRef="videoB" title="Pelajaran 1" />);

    await waitFor(() => expect(FakePlayer.instances).toHaveLength(2));
    expect(first.destroyed).toBe(true);

    const second = FakePlayer.instances[1];
    // Regression check: pre-fix, the second YT.Player was constructed on the
    // same React-owned div YT had already ripped out of the document on the
    // first mount, so it would be detached (isConnected === false) here.
    expect(second.wasConnectedAtConstruction).toBe(true);

    // The shield must still fully cover the rebuilt player with no gaps.
    expect(container.querySelector('[data-testid="video-shield"]')).not.toBeNull();
  });

  it("recovers on a later mount after the YouTube API script fails to load once", async () => {
    const { container, unmount } = render(<VideoPlayer videoRef="videoFail" title="L1" />);

    const scripts = document.head.querySelectorAll<HTMLScriptElement>('script[src*="iframe_api"]');
    const script = scripts[scripts.length - 1];
    expect(script).toBeDefined();
    script.dispatchEvent(new Event("error"));

    await waitFor(() => {
      expect(container.querySelector('[data-testid="video-shield"]')).toBeNull();
    });
    unmount();

    // A later mount (a different lesson) must not be stuck behind the
    // earlier transient failure being cached forever.
    vi.stubGlobal("YT", { Player: FakePlayer });
    const { container: retryContainer } = render(
      <VideoPlayer videoRef="videoRetry" title="L2" />,
    );

    await waitFor(() => {
      expect(retryContainer.querySelector('[data-testid="video-shield"]')).not.toBeNull();
    });
    expect(FakePlayer.instances).toHaveLength(1);
  });

  it("resets the error state for the next lesson after a real onError, and destroys the player first", async () => {
    vi.stubGlobal("YT", { Player: FakePlayer });

    const { rerender, container } = render(
      <VideoPlayer videoRef="videoBad" title="Pelajaran 1" />,
    );

    await waitFor(() => expect(FakePlayer.instances).toHaveLength(1));
    const bad = FakePlayer.instances[0];

    bad.triggerError();

    // onError must destroy the player itself — nothing else will, since the
    // state flip alone doesn't change [id, forceFallback], so the effect's
    // own cleanup never runs for this transition.
    expect(bad.destroyed).toBe(true);

    await waitFor(() => {
      expect(container.querySelector('[data-testid="video-shield"]')).toBeNull();
    });

    // Same live instance, no `key` — a different lesson must get a fresh
    // attempt at the real player, not inherit the previous lesson's fallback.
    rerender(<VideoPlayer videoRef="videoGood" title="Pelajaran 2" />);

    await waitFor(() => expect(FakePlayer.instances).toHaveLength(2));
    expect(container.querySelector('[data-testid="video-shield"]')).not.toBeNull();
  });
});
