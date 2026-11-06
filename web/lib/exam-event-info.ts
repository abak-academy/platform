export function isHttpUrl(value: string): boolean {
  if (!/^https?:\/\/[^/?#\s]/i.test(value) || /^https?:\/\/[^/?#]*@/i.test(value)) return false;
  try {
    return new URL(value).host !== "";
  } catch {
    return false;
  }
}

export function isWhatsAppGroupUrl(value: string): boolean {
  return /^https:\/\/chat\.whatsapp\.com\/+[^\s/][^\s]*$/i.test(value);
}
