import { afterEach, describe, expect, it, vi } from "vitest";
import { pageMetadata, siteUrl } from "./site-metadata";

afterEach(() => vi.unstubAllEnvs());

describe("pageMetadata", () => {
  it("emits no canonical or Open Graph URL when NEXT_PUBLIC_SITE_URL is unset", () => {
    vi.stubEnv("NEXT_PUBLIC_SITE_URL", "");
    const m = pageMetadata("/event", "Event", "desc");
    expect(siteUrl()).toBeUndefined();
    expect(m.alternates).toBeUndefined();
    expect(m.openGraph).toBeUndefined();
    expect(m.title).toBe("Event");
  });

  it("emits a relative canonical resolved against the configured site URL", () => {
    vi.stubEnv("NEXT_PUBLIC_SITE_URL", "https://abakacademy.id");
    const m = pageMetadata("/program", "Program", "desc");
    expect(siteUrl()?.origin).toBe("https://abakacademy.id");
    expect(m.alternates?.canonical).toBe("/program");
    expect(m.openGraph).toMatchObject({ url: "/program" });
  });
});
