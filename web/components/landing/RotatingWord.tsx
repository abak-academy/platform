"use client";

import { useEffect, useState } from "react";

export function RotatingWord({ words, fallback }: { words: string[]; fallback: string }) {
  const [index, setIndex] = useState(0);
  const list = [fallback, ...words];
  const key = words.join("\u0000");

  useEffect(() => {
    if (!key || window.matchMedia?.("(prefers-reduced-motion: reduce)").matches) return;
    const count = key.split("\u0000").length + 1;
    const id = setInterval(() => setIndex((i) => (i + 1) % count), 2600);
    return () => clearInterval(id);
  }, [key]);

  return (
    <span className="relative inline-grid text-brand-ink">
      <span className="sr-only">{fallback}</span>
      <span aria-hidden="true" className="inline-grid">
        {list.map((w, i) => (
          <span
            key={`${i}-${w}`}
            data-active={i === index}
            className={`col-start-1 row-start-1 transition-opacity duration-300 ${i === index ? "opacity-100" : "opacity-0"}`}
          >
            {w}
          </span>
        ))}
      </span>
    </span>
  );
}
