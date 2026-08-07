import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";

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
      events?: {
        onReady?: (e: { target: FakePlayer }) => void;
        onError?: (e: { target: FakePlayer }) => void;
        onStateChange?: (e: { data: number; target: FakePlayer }) => void;
      };
    },
  ) {
    this.wasConnectedAtConstruction = el.isConnected;
    FakePlayer.instances.push(this);
    this.iframeEl = document.createElement("iframe");
    el.replaceWith(this.iframeEl);
    // Fired from inside the constructor, and carrying `target`, exactly as the
    // real API does. Both halves matter: the timing is what makes a handler
    // reading the instance back off a ref see null, and `target` is the only
    // reference available that early.
    opts.events?.onReady?.({ target: this });
  }

  played = false;
  playVideo() {
    this.played = true;
  }
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
  // Audio state is recorded, not swallowed: a stub that accepts mute/setVolume
  // and forgets them lets a control that never reaches the player still pass.
  muted = false;
  volume = 100;
  mute() {
    this.muted = true;
  }
  unMute() {
    this.muted = false;
  }
  setVolume(v: number) {
    this.volume = v;
  }
  destroy() {
    this.destroyed = true;
    this.iframeEl.remove();
  }
  /** Simulates a real YT.Player error event (private/deleted video, region block, etc). */
  triggerError() {
    this.opts.events?.onError?.({ target: this });
  }

  /**
   * Emits a YT.PlayerState transition. playVideo() deliberately does NOT emit
   * PLAYING on its own — a real player can sit in BUFFERING or have the request
   * refused by autoplay policy, and the gate has to survive both.
   */
  emitState(data: number) {
    this.opts.events?.onStateChange?.({ data, target: this });
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

  // onReady fires from inside the YT.Player constructor, so a handler reading
  // the instance back off the ref sees null and stores 0 — which collapses the
  // scrubber's max={Math.max(duration, 1)} to 1 and clamps every seek.
  it("reads duration from onReady even when it fires during construction", async () => {
    vi.stubGlobal("YT", { Player: FakePlayer });

    render(<VideoPlayer videoRef="abc123" title="L1" />);
    await waitFor(() => expect(FakePlayer.instances).toHaveLength(1));

    // FakePlayer.getDuration() returns 100 and calls onReady synchronously.
    await waitFor(() =>
      expect(screen.getByRole("slider", { name: /posisi/i })).toHaveAttribute("max", "100"),
    );
  });

  // Measured in a real browser 2026-08-07: the shield's background is
  // rgba(0,0,0,0), so before the first play YouTube draws its title, channel,
  // logos and a "Watch on YouTube" button straight through it. Clicks were
  // blocked at every one of 2306 sampled points — this covers the pixels.
  it("covers the player with an opaque gate until the first play", async () => {
    vi.stubGlobal("YT", { Player: FakePlayer });

    render(<VideoPlayer videoRef="abc123" title="Pelajaran 1" />);
    await waitFor(() => expect(FakePlayer.instances).toHaveLength(1));

    const gate = screen.getByTestId("video-gate");
    // Opaque and theme-independent. bg-ink-900 would not do: the ink scale
    // inverts under html[data-theme="dark"], turning the panel white.
    expect(gate.className).toContain("bg-black");
    expect(gate.className).not.toContain("bg-ink-900");
    expect(gate.className).toContain("inset-0");
    // Above the shield (z-10) and the control bar (z-20).
    expect(gate.className).toContain("z-30");
  });

  // playVideo() is a request, not a guarantee. Lowering the gate on the click
  // uncovers YouTube's paused chrome whenever the request stalls in BUFFERING or
  // is refused outright — observed in a real browser as -1 → 3 → -1 with
  // currentTime still 0 under autoplay policy.
  it("keeps the gate up until the player actually reports PLAYING", async () => {
    vi.stubGlobal("YT", { Player: FakePlayer });

    render(<VideoPlayer videoRef="abc123" title="Pelajaran 1" />);
    await waitFor(() => expect(FakePlayer.instances).toHaveLength(1));
    const p = FakePlayer.instances[0];

    fireEvent.click(screen.getByTestId("video-gate"));
    expect(p.played).toBe(true);

    // Buffering, then the request falls back to unstarted — nothing is playing,
    // so the gate must still be covering the player.
    await act(async () => p.emitState(3));
    expect(screen.getByTestId("video-gate")).toBeInTheDocument();
    await act(async () => p.emitState(-1));
    expect(screen.getByTestId("video-gate")).toBeInTheDocument();

    await act(async () => p.emitState(1));
    expect(screen.queryByTestId("video-gate")).toBeNull();
  });

  // Verified in a real browser: the end screen carries the video title, the
  // Google/YouTube logos and a "Video lainnya" card linking to another video —
  // a bigger leak than the pre-play state the gate was built for.
  it("restores the gate when the video ends", async () => {
    vi.stubGlobal("YT", { Player: FakePlayer });

    render(<VideoPlayer videoRef="abc123" title="Pelajaran 1" />);
    await waitFor(() => expect(FakePlayer.instances).toHaveLength(1));
    const p = FakePlayer.instances[0];

    fireEvent.click(screen.getByTestId("video-gate"));
    await act(async () => p.emitState(1));
    expect(screen.queryByTestId("video-gate")).toBeNull();

    await act(async () => p.emitState(0));
    expect(screen.getByTestId("video-gate")).toBeInTheDocument();
  });

  // Deliberate, and measured rather than assumed: with the shield swallowing
  // every pointer event YouTube never receives the mousemove that re-shows its
  // chrome, so a paused frame stays clean. Re-covering it would hide the exact
  // frame a student paused to read.
  it("leaves the paused frame visible", async () => {
    vi.stubGlobal("YT", { Player: FakePlayer });

    render(<VideoPlayer videoRef="abc123" title="Pelajaran 1" />);
    await waitFor(() => expect(FakePlayer.instances).toHaveLength(1));
    const p = FakePlayer.instances[0];

    fireEvent.click(screen.getByTestId("video-gate"));
    await act(async () => p.emitState(1));
    await act(async () => p.emitState(2));

    expect(screen.queryByTestId("video-gate")).toBeNull();
  });

  // The gate belongs to the lesson, not the session: the next lesson starts
  // paused on its own iframe, showing its own title through the shield.
  it("restores the gate when the lesson changes", async () => {
    vi.stubGlobal("YT", { Player: FakePlayer });

    const { rerender } = render(<VideoPlayer videoRef="lessonOne" title="Pelajaran 1" />);
    await waitFor(() => expect(FakePlayer.instances).toHaveLength(1));
    fireEvent.click(screen.getByTestId("video-gate"));
    await act(async () => FakePlayer.instances[0].emitState(1));
    expect(screen.queryByTestId("video-gate")).toBeNull();

    rerender(<VideoPlayer videoRef="lessonTwo" title="Pelajaran 2" />);
    await waitFor(() => expect(FakePlayer.instances).toHaveLength(2));

    expect(screen.getByTestId("video-gate")).toBeInTheDocument();
  });

  // A click landing before onReady would dismiss the gate over a player that
  // never starts, leaving the user on YouTube's paused chrome — the exact state
  // the gate exists to prevent.
  it("does not let the gate be used before the player is ready", async () => {
    let release: (() => void) | undefined;
    class SlowPlayer extends FakePlayer {
      constructor(el: HTMLElement, opts: ConstructorParameters<typeof FakePlayer>[1]) {
        const events = opts.events;
        super(el, { ...opts, events: {} });
        release = () => events?.onReady?.({ target: this });
      }
    }
    vi.stubGlobal("YT", { Player: SlowPlayer });

    render(<VideoPlayer videoRef="abc123" title="Pelajaran 1" />);
    await waitFor(() => expect(FakePlayer.instances).toHaveLength(1));

    expect(screen.getByTestId("video-gate")).toBeDisabled();

    await act(async () => {
      release?.();
    });
    expect(screen.getByTestId("video-gate")).toBeEnabled();
  });

  // Fullscreening the iframe would hand the whole viewport to YouTube's own
  // chrome and undo the shield entirely — the one action that can defeat it
  // without touching a line of the shield's own code.
  it("fullscreens the wrapper, never the iframe", async () => {
    vi.stubGlobal("YT", { Player: FakePlayer });
    const targets: string[] = [];
    Object.defineProperty(HTMLElement.prototype, "requestFullscreen", {
      configurable: true,
      writable: true,
      value: function (this: HTMLElement) {
        targets.push(this.tagName + ":" + (this.getAttribute("data-testid") ?? this.className));
        return Promise.resolve();
      },
    });

    render(<VideoPlayer videoRef="abc123" title="L1" />);
    await waitFor(() => expect(FakePlayer.instances).toHaveLength(1));

    fireEvent.click(screen.getByRole("button", { name: /layar penuh/i }));

    expect(targets).toHaveLength(1);
    expect(targets[0]).not.toContain("IFRAME");
    // The wrapper is the positioned ancestor the shield is inset-0 against.
    expect(targets[0]).toContain("relative");

    delete (HTMLElement.prototype as Partial<HTMLElement>).requestFullscreen;
  });

  it("drives the player's volume from the volume slider", async () => {
    vi.stubGlobal("YT", { Player: FakePlayer });

    render(<VideoPlayer videoRef="abc123" title="L1" />);
    await waitFor(() => expect(FakePlayer.instances).toHaveLength(1));

    const slider = screen.getByRole("slider", { name: /volume/i });
    fireEvent.change(slider, { target: { value: "40" } });

    // Asserted on the player, not the input: mute/unMute alone cannot express
    // an intermediate level, which is the gap this control closes.
    expect(FakePlayer.instances[0].volume).toBe(40);
    expect(slider).toHaveValue("40");
  });

  it("mutes at zero volume and unmutes when raised again", async () => {
    vi.stubGlobal("YT", { Player: FakePlayer });

    render(<VideoPlayer videoRef="abc123" title="L1" />);
    await waitFor(() => expect(FakePlayer.instances).toHaveLength(1));
    const p = FakePlayer.instances[0];
    const slider = screen.getByRole("slider", { name: /volume/i });

    fireEvent.change(slider, { target: { value: "0" } });
    expect(p.muted).toBe(true);
    expect(screen.getByRole("button", { name: /bunyikan/i })).toBeInTheDocument();

    fireEvent.change(slider, { target: { value: "55" } });
    expect(p.muted).toBe(false);
    expect(p.volume).toBe(55);
  });

  it("gives an audible level back when unmuting a slider left at zero", async () => {
    vi.stubGlobal("YT", { Player: FakePlayer });

    render(<VideoPlayer videoRef="abc123" title="L1" />);
    await waitFor(() => expect(FakePlayer.instances).toHaveLength(1));
    const p = FakePlayer.instances[0];

    fireEvent.change(screen.getByRole("slider", { name: /volume/i }), {
      target: { value: "0" },
    });
    fireEvent.click(screen.getByRole("button", { name: /bunyikan/i }));

    expect(p.muted).toBe(false);
    expect(p.volume).toBeGreaterThan(0);
  });

  // Every control is bound to the instance destroyed on a lesson switch, so any
  // value carried over describes a player that no longer exists.
  it("resets per-video control state when the lesson changes", async () => {
    vi.stubGlobal("YT", { Player: FakePlayer });

    const { rerender } = render(<VideoPlayer videoRef="lessonOne" title="Pelajaran 1" />);
    await waitFor(() => expect(FakePlayer.instances).toHaveLength(1));

    const volumeSlider = screen.getByRole("slider", { name: /volume/i });
    fireEvent.change(volumeSlider, { target: { value: "10" } });
    fireEvent.click(screen.getByRole("button", { name: /bisukan/i }));
    fireEvent.change(screen.getByRole("slider", { name: /posisi/i }), {
      target: { value: "42" },
    });

    expect(screen.getByRole("slider", { name: /posisi/i })).toHaveValue("42");

    rerender(<VideoPlayer videoRef="lessonTwo" title="Pelajaran 2" />);
    await waitFor(() => expect(FakePlayer.instances).toHaveLength(2));

    expect(screen.getByRole("slider", { name: /posisi/i })).toHaveValue("0");
    expect(screen.getByRole("slider", { name: /volume/i })).toHaveValue("100");
    // A stale mute flag would leave this button showing "unmute" over a player
    // that is in fact audible.
    expect(screen.getByRole("button", { name: /bisukan/i })).toBeInTheDocument();
  });
});
