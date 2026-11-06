export function stripHtmlToPlainText(html?: string | null): string {
  if (!html) return "";
  const d = document.createElement("div");
  d.innerHTML = html;
  d.querySelectorAll("script, style").forEach((el) => el.remove());
  return (d.textContent || "").replace(/\s+/g, " ").trim();
}

export function materialHTMLOrNull(html?: string | null): string | null {
  if (!html?.trim()) return null;
  const content = document.createElement("div");
  content.innerHTML = html;
  if (!(content.textContent ?? "").trim() && !content.querySelector("img, table")) return null;
  return html.trim();
}
