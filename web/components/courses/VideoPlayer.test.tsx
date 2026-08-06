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
    public opts: { events?: { onReady?: () => void; onError?: () => void } },
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
  setVolume() {}
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

  it("falls back to a plain embed when the IFrame API fails to load", async () => {
    const { container } = render(
      <VideoPlayer videoRef="abc123" title="L1" forceFallback />,
    );
    const iframe = container.querySelector("iframe");
    expect(iframe).not.toBeNull();
    expect(container.querySelector('[data-testid="video-shield"]')).toBeNull();
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

  it("resets fallback for the next lesson after a real onError, and destroys the player before falling back", async () => {
    vi.stubGlobal("YT", { Player: FakePlayer });

    const { rerender, container } = render(
      <VideoPlayer videoRef="videoBad" title="Pelajaran 1" />,
    );

    await waitFor(() => expect(FakePlayer.instances).toHaveLength(1));
    const bad = FakePlayer.instances[0];

    bad.triggerError();

    // onError must destroy the player itself — nothing else will, since
    // setFallback alone doesn't change [id, forceFallback], so the effect's
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
