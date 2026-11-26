"use client";

import { useEffect } from "react";

const EFFECTS = ["pm-up", "pm-left", "pm-right", "pm-zoom", "pm-blur", "pm-tilt"];
const SWARM_VARS = ["--color-brand-600", "--color-ember-500", "--color-gold-600", "--color-teal-600"];
const SWARM_LIGHT = "rgba(255,255,255,.85)";

function shuffled<T>(a: T[]): T[] {
  return [...a].sort(() => Math.random() - 0.5);
}

export function PublicMotion() {
  useEffect(() => {
    if (
      window.matchMedia("(prefers-reduced-motion: reduce)").matches ||
      window.matchMedia("(pointer: coarse)").matches
    ) {
      return;
    }

    const order = shuffled(EFFECTS);
    let sectionCount = 0;
    const seen = new WeakSet<Element>();
    const unseenSections = new Set<Element>();

    const io =
      "IntersectionObserver" in window
        ? new IntersectionObserver(
            (entries) => {
              for (const en of entries) {
                if (!en.isIntersecting) continue;
                const el = en.target;
                el.classList.add("pm-in");
                io!.unobserve(el);
                unseenSections.delete(el);
                setTimeout(() => el.classList.add("pm-done"), 1400);
              }
            },
            { threshold: 0.12, rootMargin: "0px 0px -8% 0px" },
          )
        : null;

    const decorate = (el: Element) => {
      const head = el.querySelector("h1, h2, h3");
      const kids: Element[] = [];
      if (head) {
        kids.push(head);
        (head as HTMLElement).style.setProperty("--pm-d", "0ms");
      }
      const grid = el.querySelector("[data-reveal-grid]");
      if (grid) {
        [...grid.children].slice(0, 12).forEach((c, k) => {
          kids.push(c);
          (c as HTMLElement).style.setProperty("--pm-d", `${120 + k * 80}ms`);
        });
      }
      kids.forEach((k) => k.classList.add("pm-kid"));
    };

    const scan = () => {
      if (!io) return;
      document.querySelectorAll("[data-reveal]").forEach((el) => {
        if (seen.has(el)) return;
        seen.add(el);
        if (el.getBoundingClientRect().top < window.innerHeight * 0.9) return;
        el.classList.add("pm-reveal", order[sectionCount++ % order.length]);
        decorate(el);
        io.observe(el);
        unseenSections.add(el);
      });
      document.querySelectorAll("[data-reveal]:not(.pm-in) [data-reveal-grid]").forEach((g) => {
        [...g.children].slice(0, 12).forEach((c, k) => {
          if (c.classList.contains("pm-kid")) return;
          c.classList.add("pm-kid");
          (c as HTMLElement).style.setProperty("--pm-d", `${120 + k * 80}ms`);
        });
      });
    };

    const mo = new MutationObserver(scan);
    mo.observe(document.body, { childList: true, subtree: true });
    scan();

    const canvas = document.createElement("canvas");
    canvas.id = "pm-swarm";
    canvas.setAttribute("aria-hidden", "true");
    document.body.appendChild(canvas);
    const ctx = canvas.getContext?.("2d") ?? null;
    const fit = () => {
      const d = window.devicePixelRatio || 1;
      canvas.width = window.innerWidth * d;
      canvas.height = window.innerHeight * d;
      ctx?.setTransform(d, 0, 0, d, 0, 0);
    };
    fit();

    const colors = SWARM_VARS.map((v) => getComputedStyle(document.documentElement).getPropertyValue(v).trim());
    type P = { ox: number; oy: number; x: number; y: number; vx: number; vy: number; k: number; c: string };
    let ps: P[] = [];
    let mx = 0, my = 0, px = 0, py = 0;
    let light = false;
    let raf = 0;
    let heroDirty = true;
    let tilted: Element | null = null;

    const heroPhoto = () => document.querySelector<HTMLElement>("[data-hero-photo]");
    const heroContent = () => document.querySelector<HTMLElement>("[data-hero-content]");

    const heroStep = () => {
      heroDirty = false;
      const photo = heroPhoto();
      const hero = photo?.closest("[data-hero]") ?? photo;
      if (!photo || !hero) return;
      const h = Math.max(hero.getBoundingClientRect().height, 1);
      const y = Math.min(window.scrollY, h);
      photo.style.transform = `translate3d(0, ${(y * 0.35).toFixed(1)}px, 0)`;
      const c = heroContent();
      if (c) {
        c.style.opacity = String(Math.max(0, 1 - (y / h) * 1.3).toFixed(3));
        c.style.transform = `translateY(${(y * 0.14).toFixed(1)}px)`;
      }
    };

    const tick = () => {
      raf = 0;
      if (heroDirty) heroStep();
      if (!ctx || !ps.length) return;
      ctx.clearRect(0, 0, window.innerWidth, window.innerHeight);
      const sp = Math.hypot(mx - px, my - py);
      px = mx;
      py = my;
      let moving = sp > 0.3;
      for (const p of ps) {
        if (sp > 6) {
          p.vx += (Math.random() - 0.5) * sp * 0.35;
          p.vy += (Math.random() - 0.5) * sp * 0.35;
        }
        p.vx = (p.vx + (mx + 14 + p.ox - p.x) * p.k) * 0.82;
        p.vy = (p.vy + (my + 14 + p.oy - p.y) * p.k) * 0.82;
        p.x += p.vx;
        p.y += p.vy;
        if (Math.abs(p.vx) + Math.abs(p.vy) > 0.05) moving = true;
        ctx.fillStyle = light ? SWARM_LIGHT : p.c;
        ctx.fillRect(Math.round(p.x), Math.round(p.y), 4, 4);
      }
      if (moving) raf = requestAnimationFrame(tick);
    };
    const kick = () => {
      if (!raf) raf = requestAnimationFrame(tick);
    };

    const setTilt = (card: Element | null, e?: PointerEvent) => {
      if (tilted && tilted !== card) {
        tilted.classList.remove("pm-tilting");
        (tilted as HTMLElement).style.removeProperty("--pm-tx");
        (tilted as HTMLElement).style.removeProperty("--pm-ty");
      }
      tilted = card;
      if (!card || !e) return;
      const r = card.getBoundingClientRect();
      card.classList.add("pm-tilting");
      (card as HTMLElement).style.setProperty("--pm-tx", (((e.clientX - r.left) / r.width - 0.5) * 8).toFixed(2) + "deg");
      (card as HTMLElement).style.setProperty("--pm-ty", ((0.5 - (e.clientY - r.top) / r.height) * 8).toFixed(2) + "deg");
    };

    const onMove = (e: PointerEvent) => {
      if (e.pointerType && e.pointerType !== "mouse") return;
      if (!ps.length) {
        ps = Array.from({ length: 25 }, (_, k) => ({
          ox: ((k % 5) - 2) * 6,
          oy: (Math.floor(k / 5) - 2) * 6,
          x: e.clientX,
          y: e.clientY,
          vx: 0,
          vy: 0,
          k: 0.035 + Math.random() * 0.09,
          c: colors[k % colors.length],
        }));
        px = e.clientX;
        py = e.clientY;
      }
      mx = e.clientX;
      my = e.clientY;
      const t = e.target as Element | null;
      light = !!t?.closest?.("[data-swarm-light]");
      const card = t?.closest?.("[data-tilt]") ?? null;
      setTilt(card?.closest("[data-reveal]")?.classList.contains("pm-done") ? card : null, e);
      kick();
    };
    const onScroll = () => {
      heroDirty = true;
      kick();
    };
    const onLeave = () => ctx?.clearRect(0, 0, window.innerWidth, window.innerHeight);
    const onResize = () => {
      fit();
      heroDirty = true;
    };

    window.addEventListener("pointermove", onMove, { passive: true });
    window.addEventListener("scroll", onScroll, { passive: true });
    window.addEventListener("resize", onResize, { passive: true });
    document.documentElement.addEventListener("pointerleave", onLeave, { passive: true });
    kick();

    return () => {
      window.removeEventListener("pointermove", onMove);
      window.removeEventListener("scroll", onScroll);
      window.removeEventListener("resize", onResize);
      document.documentElement.removeEventListener("pointerleave", onLeave);
      if (raf) cancelAnimationFrame(raf);
      mo.disconnect();
      io?.disconnect();
      canvas.remove();
      setTilt(null);
      unseenSections.forEach((el) => el.classList.remove("pm-reveal", "pm-in", ...EFFECTS));
      document.querySelectorAll(".pm-kid").forEach((k) => k.classList.remove("pm-kid"));
      const photo = heroPhoto();
      if (photo) photo.style.transform = "";
      const c = heroContent();
      if (c) {
        c.style.opacity = "";
        c.style.transform = "";
      }
    };
  }, []);

  return null;
}
