"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { Maximize, Pause, PlayCircle, Play, Volume2, VolumeX } from "lucide-react";

interface VideoPlayerProps {
  videoRef?: string;
  title?: string;
  /** Test seam: render the no-JS-API fallback without stubbing the network. */
  forceFallback?: boolean;
}

function toYoutubeId(value?: string): string | null {
  if (!value) return null;
  const trimmed = value.trim();
  if (!trimmed) return null;
  if (/^https?:\/\//i.test(trimmed)) {
    try {
      const url = new URL(trimmed);
      if (/youtube\.com$/i.test(url.hostname)) {
        // /watch?v=VIDEO_ID
        const v = url.searchParams.get("v");
        if (v) return v;
        // /shorts/VIDEO_ID  or  /embed/VIDEO_ID
        const match = url.pathname.match(/\/(shorts|embed)\/([^/?]+)/);
        if (match?.[2]) return match[2];
      }
      if (/youtu\.be$/i.test(url.hostname)) {
        const id = url.pathname.replace(/^\//, "").split("?")[0];
        if (id) return id;
      }
      return null;
    } catch {
      return null;
    }
  }
  return trimmed;
}

interface YTPlayer {
  playVideo(): void;
  pauseVideo(): void;
  seekTo(seconds: number, allowSeekAhead: boolean): void;
  getCurrentTime(): number;
  getDuration(): number;
  getVideoLoadedFraction(): number;
  mute(): void;
  unMute(): void;
  setVolume(volume: number): void;
  destroy(): void;
}

type YTGlobal = {
  YT?: { Player: new (el: HTMLElement, opts: unknown) => YTPlayer };
  onYouTubeIframeAPIReady?: () => void;
};

// YT.PlayerState members we act on.
const YT_ENDED = 0;
const YT_PLAYING = 1;

// One script tag per page, shared by every player instance on it.
let apiPromise: Promise<void> | null = null;

function loadYoutubeApi(): Promise<void> {
  if (apiPromise) return apiPromise;
  apiPromise = new Promise<void>((resolve, reject) => {
    const w = window as unknown as YTGlobal;
    if (w.YT?.Player) {
      resolve();
      return;
    }
    const previous = w.onYouTubeIframeAPIReady;
    w.onYouTubeIframeAPIReady = () => {
      previous?.();
      resolve();
    };
    const script = document.createElement("script");
    script.src = "https://www.youtube.com/iframe_api";
    script.async = true;
    script.onerror = () => reject(new Error("youtube iframe_api failed to load"));
    document.head.appendChild(script);
  }).catch((err) => {
    // A transient failure (flaky network, ad blocker) must not be cached
    // forever — reset so the next mount gets a fresh attempt instead of
    // permanently falling back to the shield-less embed.
    apiPromise = null;
    throw err;
  });
  return apiPromise;
}

function formatTime(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds < 0) return "0:00";
  const m = Math.floor(seconds / 60);
  const s = Math.floor(seconds % 60);
  return `${m}:${String(s).padStart(2, "0")}`;
}

/** In-app 16:9 notice card. Carries no YouTube iframe, so it cannot leak out. */
function PlayerNotice({ title, message }: { title?: string; message: string }) {
  return (
    <div
      className="overflow-hidden rounded-lg border border-line bg-black"
      style={{ aspectRatio: "16 / 9" }}
    >
      {/* Same reason as the gate and the control scrim: this card stands in for
          the video, and the ink scale inverts under html[data-theme="dark"].
          On ink-900 the title measured 2.92:1 in dark mode, under AA's 4.5.
          (text-ink-300 was also dead — the scale has no 300 step — so the title
          was only legible by inheriting its container's colour.) */}
      <div className="flex size-full flex-col items-center justify-center gap-3 text-white/70">
        <PlayCircle size={48} strokeWidth={1.5} />
        <div className="text-center">
          <p className="text-sm font-medium text-white">
            {title ? `${title}` : "Video pelajaran"}
          </p>
          <p className="mt-1 text-xs text-white/70">{message}</p>
        </div>
      </div>
    </div>
  );
}

export function VideoPlayer({ videoRef, title, forceFallback }: VideoPlayerProps) {
  const id = toYoutubeId(videoRef);

  const wrapperEl = useRef<HTMLDivElement | null>(null);
  const mountEl = useRef<HTMLDivElement | null>(null);
  const player = useRef<YTPlayer | null>(null);

  const [fallback, setFallback] = useState(Boolean(forceFallback));
  const [unplayable, setUnplayable] = useState(false);
  const [playing, setPlaying] = useState(false);
  const [muted, setMuted] = useState(false);
  const [position, setPosition] = useState(0);
  const [duration, setDuration] = useState(0);
  const [loaded, setLoaded] = useState(0);
  const [volume, setVolume] = useState(100);
  const [ready, setReady] = useState(false);
  // YouTube only auto-hides its chrome while the video is playing. Verified in a
  // browser 2026-08-07: before the first play it renders the video title, the
  // channel, its logos and a "Watch on YouTube" button straight through the
  // shield, which is transparent. This gate covers that window.
  const [started, setStarted] = useState(false);

  useEffect(() => {
    // A lesson switch (no `key` at the call site, so this is a live-instance
    // prop change, not a remount) must give the new video a fresh attempt at
    // the real player — otherwise one bad video_url pins every later lesson
    // in the session to the shield-less fallback. forceFallback still wins.
    setFallback(Boolean(forceFallback));
    setUnplayable(false);
    // Every control below is bound to the player instance destroyed on cleanup.
    // A fresh YT.Player starts paused at 0s, unmuted, at full volume, so any
    // value kept from the previous lesson would describe a player that no
    // longer exists — a stale scrubber position, or a mute button whose icon
    // is the inverse of what the new video is actually doing.
    setPlaying(false);
    setPosition(0);
    setDuration(0);
    setLoaded(0);
    setMuted(false);
    setVolume(100);
    setReady(false);
    setStarted(false);
    if (!id || forceFallback) return;
    let cancelled = false;

    loadYoutubeApi()
      .then(() => {
        if (cancelled || !mountEl.current) return;
        const w = window as unknown as YTGlobal;
        if (!w.YT?.Player) throw new Error("YT.Player missing after ready");
        // YT.Player REPLACES the element it's given with its own <iframe> —
        // it does not mount inside it, and destroy() does not restore what
        // was there. mountEl.current is React's node and must never be
        // handed to YT directly, or the second video on this instance gets
        // constructed against an already-detached div. Give YT a throwaway
        // child instead, rebuilt fresh on every id change.
        const mountNode = document.createElement("div");
        mountEl.current.appendChild(mountNode);
        // Handlers below take the player from `e.target`, never from
        // player.current: onReady can fire from inside this constructor, when
        // the ref is still null and a `const` local would still be in its
        // temporal dead zone. e.target is the instance the API hands back.
        player.current = new w.YT.Player(mountNode, {
          videoId: id,
          // Without these the IFrame API writes its 640x390 default onto the
          // generated iframe, which the responsive 16:9 wrapper then crops.
          width: "100%",
          height: "100%",
          playerVars: {
            enablejsapi: 1,
            controls: 0,
            rel: 0,
            disablekb: 1,
            playsinline: 1,
            // No modestbranding — YouTube retired it. The shield is what
            // removes the branding; a dead parameter here would only suggest
            // otherwise to the next reader.
            origin: window.location.origin,
          },
          host: "https://www.youtube-nocookie.com",
          events: {
            onReady: (e: { target: YTPlayer }) => {
              if (cancelled) return;
              setDuration(e.target.getDuration());
              setReady(true);
            },
            onStateChange: (e: { data: number; target: YTPlayer }) => {
              if (cancelled) return;
              setPlaying(e.data === YT_PLAYING);
              setDuration(e.target.getDuration());
              // The gate follows the player's real state, never the click.
              // Measured in a browser 2026-08-07/08: chrome is visible while
              // unstarted or buffering, hidden while playing, and back in force
              // on ENDED — the end screen carries the title, the logos and a
              // "Video lainnya" card linking to another video.
              if (e.data === YT_PLAYING) setStarted(true);
              else if (e.data === YT_ENDED) setStarted(false);
              // PAUSED is deliberately absent: with the shield swallowing every
              // pointer event YouTube never gets the mousemove that would
              // re-show its chrome, so a paused frame stays clean — verified.
              // Re-covering it would hide the very frame a student paused to read.
            },
            // A per-video failure (embedding disabled, deleted, region-locked)
            // must NOT reach the shield-less fallback: YouTube's own error
            // screen carries a live "Watch on YouTube" link, which is the exact
            // leak this player exists to close. D7 covers the API failing to
            // load, not one bad video_url.
            onError: (e: { target: YTPlayer }) => {
              if (cancelled) return;
              // e.target, not player.current — an error raised during
              // construction would otherwise leave the iframe alive and
              // YouTube's "Watch on YouTube" link with it.
              e.target.destroy();
              player.current = null;
              if (mountEl.current) mountEl.current.innerHTML = "";
              setUnplayable(true);
            },
          },
        });
      })
      .catch(() => {
        if (!cancelled) setFallback(true);
      });

    return () => {
      cancelled = true;
      player.current?.destroy();
      player.current = null;
      if (mountEl.current) mountEl.current.innerHTML = "";
    };
  }, [id, forceFallback]);

  useEffect(() => {
    if (!playing) return;
    const tick = setInterval(() => {
      const p = player.current;
      if (!p) return;
      setPosition(p.getCurrentTime());
      setLoaded(p.getVideoLoadedFraction());
    }, 250);
    return () => clearInterval(tick);
  }, [playing]);

  const togglePlay = useCallback(() => {
    const p = player.current;
    if (!p) return;
    if (playing) p.pauseVideo();
    else p.playVideo();
  }, [playing]);

  const toggleMute = useCallback(() => {
    const p = player.current;
    if (!p) return;
    if (muted) {
      p.unMute();
      // Unmuting a slider parked at zero would stay silent and read as a broken
      // button, so give it an audible level back.
      if (volume === 0) {
        p.setVolume(100);
        setVolume(100);
      }
    } else {
      p.mute();
    }
    setMuted(!muted);
  }, [muted, volume]);

  const changeVolume = useCallback(
    (next: number) => {
      const p = player.current;
      if (!p) return;
      p.setVolume(next);
      setVolume(next);
      // Moving the slider is an explicit audio intent: off zero must unmute, and
      // dragging to zero is how you mute without going for the button.
      if (next === 0 && !muted) {
        p.mute();
        setMuted(true);
      } else if (next > 0 && muted) {
        p.unMute();
        setMuted(false);
      }
    },
    [muted],
  );

  // Deliberately does not lower the gate — onStateChange does that, and only on
  // PLAYING. playVideo() is a request, not a guarantee: it can sit in BUFFERING,
  // or be refused outright by autoplay policy (observed in a real browser as
  // -1 → 3 → -1 with currentTime still 0). Lowering the gate here would uncover
  // YouTube's paused chrome in exactly those cases.
  const startPlayback = useCallback(() => {
    player.current?.playVideo();
  }, []);

  const seek = useCallback((next: number) => {
    player.current?.seekTo(next, true);
    setPosition(next);
  }, []);

  // Fullscreen targets the wrapper, never the iframe: fullscreening the iframe
  // would hand the viewport to YouTube's chrome and undo the shield.
  const goFullscreen = useCallback(() => {
    wrapperEl.current?.requestFullscreen?.();
  }, []);

  if (!id) {
    return (
      <PlayerNotice
        title={title}
        message="Video belum tersedia. Hubungi admin untuk informasi lebih lanjut."
      />
    );
  }

  if (unplayable) {
    return (
      <PlayerNotice
        title={title}
        message="Video tidak dapat diputar. Hubungi admin untuk informasi lebih lanjut."
      />
    );
  }

  if (fallback) {
    return (
      <div
        className="overflow-hidden rounded-lg border border-line bg-ink-900"
        style={{ aspectRatio: "16 / 9" }}
      >
        <iframe
          title={title ?? "Lesson video"}
          src={`https://www.youtube-nocookie.com/embed/${encodeURIComponent(id)}?rel=0`}
          allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture"
          allowFullScreen
          className="block size-full border-0"
        />
      </div>
    );
  }

  return (
    <div
      ref={wrapperEl}
      className="relative overflow-hidden rounded-lg border border-line bg-ink-900"
      style={{ aspectRatio: "16 / 9" }}
    >
      {/* The IFrame API writes width/height ATTRIBUTES on the iframe it
          generates. Author CSS outranks presentational attributes, so this
          child rule is what actually sizes it — the width/height player
          options above are the belt to this pair of braces. */}
      <div
        ref={mountEl}
        data-testid="video-mount"
        className="block size-full [&>iframe]:block [&>iframe]:size-full [&>iframe]:border-0"
      />

      {/* Absorbs every pointer event, so nothing YouTube draws is reachable.
          Must have no gaps. It is transparent, so it stops the click but not the
          pixels — the gate below is what hides them before playback starts. */}
      <div
        data-testid="video-shield"
        className="absolute inset-0 z-10"
        onContextMenu={(e) => e.preventDefault()}
      />

      {/* Opaque until the first play. Sits above the control bar too: while the
          video has never started there is no position to scrub and no state to
          mute, so the only meaningful action is to begin. Its label avoids
          "Putar" — the control bar's play/pause toggle owns that name, and two
          buttons answering to it is ambiguous to a screen reader. */}
      {!started && (
        <button
          type="button"
          data-testid="video-gate"
          onClick={startPlayback}
          disabled={!ready}
          aria-label={title ? `Mulai video: ${title}` : "Mulai video"}
          // bg-black, not bg-ink-900: the ink scale inverts under
          // html[data-theme="dark"] (ink-900 becomes #eeeffc), which would turn
          // this panel white and hide the white icon on it. A video surface is
          // dark in both themes.
          className="absolute inset-0 z-30 flex size-full flex-col items-center justify-center gap-3 bg-black text-white disabled:cursor-progress"
        >
          <PlayCircle size={56} strokeWidth={1.5} className="text-white" />
          {title && <span className="px-6 text-center text-sm font-medium">{title}</span>}
        </button>
      )}

      {/* from-black, not from-ink-900: this scrim exists so the white controls
          read over bright video, but the ink scale inverts under
          html[data-theme="dark"] (ink-900 becomes #eeeffc) — which turned the
          scrim light and the white controls on it invisible. Video chrome is
          dark in both themes. */}
      <div className="absolute inset-x-0 bottom-0 z-20 flex items-center gap-3 bg-gradient-to-t from-black/90 to-transparent px-3 pb-2 pt-6">
        <button
          type="button"
          onClick={togglePlay}
          aria-label={playing ? "Jeda" : "Putar"}
          className="shrink-0 text-white"
        >
          {playing ? <Pause className="size-5" /> : <Play className="size-5" />}
        </button>

        <span className="shrink-0 font-mono text-[11px] text-white/80">
          {formatTime(position)}
        </span>

        <div className="relative flex-1">
          <div
            className="pointer-events-none absolute inset-y-1/2 h-1 -translate-y-1/2 rounded bg-white/40"
            style={{ width: `${loaded * 100}%` }}
          />
          <input
            type="range"
            aria-label="Posisi video"
            min={0}
            max={Math.max(duration, 1)}
            step={1}
            value={position}
            onChange={(e) => seek(Number(e.target.value))}
            className="relative w-full accent-brand-600"
          />
        </div>

        <span className="shrink-0 font-mono text-[11px] text-white/80">
          {formatTime(duration)}
        </span>

        <button
          type="button"
          onClick={toggleMute}
          aria-label={muted ? "Bunyikan" : "Bisukan"}
          className="shrink-0 text-white"
        >
          {muted ? <VolumeX className="size-5" /> : <Volume2 className="size-5" />}
        </button>

        <input
          type="range"
          aria-label="Volume"
          min={0}
          max={100}
          step={1}
          value={muted ? 0 : volume}
          onChange={(e) => changeVolume(Number(e.target.value))}
          className="w-16 shrink-0 accent-brand-600"
        />

        <button
          type="button"
          onClick={goFullscreen}
          aria-label="Layar penuh"
          className="shrink-0 text-white"
        >
          <Maximize className="size-5" />
        </button>
      </div>
    </div>
  );
}
