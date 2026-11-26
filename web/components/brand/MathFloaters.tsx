const FLOATERS = [
  { glyph: "∑", left: "2%", top: "8%", rotate: -8, tone: "text-brand-600/25" },
  { glyph: "π", left: "93%", top: "6%", rotate: 11, tone: "text-teal-600/25" },
  { glyph: "∞", left: "4%", top: "46%", rotate: -14, tone: "text-gold-600/30" },
  { glyph: "√", left: "91%", top: "40%", rotate: 17, tone: "text-sky-600/25" },
];

export function MathFloaters({ light = false }: { light?: boolean }) {
  return (
    <div aria-hidden="true" className="pointer-events-none absolute inset-0 overflow-hidden">
      {FLOATERS.map((f) => (
        <span
          key={f.glyph}
          className={`absolute font-display text-6xl font-extrabold select-none ${light ? "text-white/20" : f.tone}`}
          style={{ left: f.left, top: f.top, transform: `rotate(${f.rotate}deg)` }}
        >
          {f.glyph}
        </span>
      ))}
    </div>
  );
}
