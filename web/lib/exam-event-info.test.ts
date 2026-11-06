import { describe, it, expect } from "vitest";
import { isHttpUrl, isWhatsAppGroupUrl } from "./exam-event-info";

describe("isWhatsAppGroupUrl", () => {
  it.each([
    "https://chat.whatsapp.com/AbC123",
    "https://CHAT.WhatsApp.com/AbC123",
  ])("accepts %s", (v) => expect(isWhatsAppGroupUrl(v)).toBe(true));

  it.each([
    "http://chat.whatsapp.com/Abc",
    "https://chat.whatsapp.com",
    "https://chat.whatsapp.com/",
    "https://chat.whatsapp.com:443/Abc",
    "https://chat.whatsapp.com:8443/Abc",
    "https:chat.whatsapp.com/Abc",
    "https:/chat.whatsapp.com/Abc",
    "https://x@chat.whatsapp.com/Abc",
    "https://evil.chat.whatsapp.com/Abc",
    "https://chat.whatsapp.com.evil.com/Abc",
    "https://chat.whatsapp.com/ Abc",
    "javascript:alert(1)",
    "",
  ])("rejects %s", (v) => expect(isWhatsAppGroupUrl(v)).toBe(false));
});

describe("isHttpUrl", () => {
  it.each(["http://example.com/x", "https://example.com:8443/x", "HTTPS://example.com"])("accepts %s", (v) =>
    expect(isHttpUrl(v)).toBe(true),
  );

  it.each([
    "javascript:alert(1)",
    "data:text/html,x",
    "ftp://example.com/x",
    "https:example.com/x",
    "https:/example.com/x",
    "https://user:pw@example.com/x",
    "https://user@example.com/x",
    "https:///x",
    "/relatif",
    "",
  ])("rejects %s", (v) => expect(isHttpUrl(v)).toBe(false));
});
