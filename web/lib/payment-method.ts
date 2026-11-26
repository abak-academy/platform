// Midtrans `payment_type` values as stored in orders.payment_method; the bank or card brand is not stored.
const LABELS: Record<string, string | { id: string; en: string }> = {
  gopay: "GoPay",
  shopeepay: "ShopeePay",
  dana: "DANA",
  qris: "QRIS",
  other_qris: "QRIS",
  akulaku: "Akulaku",
  kredivo: "Kredivo",
  echannel: "Mandiri Bill",
  bank_transfer: { id: "Transfer bank (Virtual Account)", en: "Bank transfer (Virtual Account)" },
  credit_card: { id: "Kartu kredit/debit", en: "Credit/debit card" },
  cstore: { id: "Gerai retail (Indomaret/Alfamart)", en: "Retail store (Indomaret/Alfamart)" },
};

export function paymentMethodLabel(code: string | undefined, lang: string): string {
  if (!code) return "—";
  const label = LABELS[code];
  if (!label) return code;
  return typeof label === "string" ? label : lang === "en" ? label.en : label.id;
}
