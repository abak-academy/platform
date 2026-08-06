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
  setVolume(volume: number): void;
  mute(): void;
  unMute(): void;
  destroy(): void;
}

type YTGlobal = {
  YT?: { Player: new (el: HTMLElement, opts: unknown) => YTPlayer };
  onYouTubeIframeAPIReady?: () => void;
};

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
  });
  return apiPromise;
}

function formatTime(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds < 0) return "0:00";
  const m = Math.floor(seconds / 60);
  const s = Math.floor(seconds % 60);
  return `${m}:${String(s).padStart(2, "0")}`;
}

export function VideoPlayer({ videoRef, title, forceFallback }: VideoPlayerProps) {
  const id = toYoutubeId(videoRef);

  const wrapperEl = useRef<HTMLDivElement | null>(null);
  const mountEl = useRef<HTMLDivElement | null>(null);
  const player = useRef<YTPlayer | null>(null);

  const [fallback, setFallback] = useState(Boolean(forceFallback));
  const [playing, setPlaying] = useState(false);
  const [muted, setMuted] = useState(false);
  const [position, setPosition] = useState(0);
  const [duration, setDuration] = useState(0);
  const [loaded, setLoaded] = useState(0);

  useEffect(() => {
    if (!id || forceFallback) return;
    let cancelled = false;

    loadYoutubeApi()
      .then(() => {
        if (cancelled || !mountEl.current) return;
        const w = window as unknown as YTGlobal;
        if (!w.YT?.Player) throw new Error("YT.Player missing after ready");
        player.current = new w.YT.Player(mountEl.current, {
          videoId: id,
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
            onReady: () => {
              if (cancelled) return;
              setDuration(player.current?.getDuration() ?? 0);
            },
            // YT.PlayerState.PLAYING === 1
            onStateChange: (e: { data: number }) => {
              if (cancelled) return;
              setPlaying(e.data === 1);
              setDuration(player.current?.getDuration() ?? 0);
            },
            onError: () => {
              if (!cancelled) setFallback(true);
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
    if (muted) p.unMute();
    else p.mute();
    setMuted(!muted);
  }, [muted]);

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
      <div
        className="overflow-hidden rounded-lg border border-line bg-ink-900"
        style={{ aspectRatio: "16 / 9" }}
      >
        <div className="flex size-full flex-col items-center justify-center gap-3 text-ink-400">
          <PlayCircle size={48} strokeWidth={1.5} />
          <div className="text-center">
            <p className="text-sm font-medium text-ink-300">
              {title ? `${title}` : "Video pelajaran"}
            </p>
            <p className="mt-1 text-xs text-ink-500">
              Video belum tersedia. Hubungi admin untuk informasi lebih lanjut.
            </p>
          </div>
        </div>
      </div>
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
      <div ref={mountEl} className="block size-full" />

      {/* Absorbs every pointer event so YouTube's title bar and logo never
          render, let alone become clickable. Must have no gaps. */}
      <div
        data-testid="video-shield"
        className="absolute inset-0 z-10"
        onContextMenu={(e) => e.preventDefault()}
      />

      <div className="absolute inset-x-0 bottom-0 z-20 flex items-center gap-3 bg-gradient-to-t from-ink-900/90 to-transparent px-3 pb-2 pt-6">
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
