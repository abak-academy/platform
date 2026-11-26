import { afterEach, describe, expect, it, vi } from "vitest";
import { MAX_PHOTO_BYTES, cropRect, preparePhoto } from "./crop-photo";

function file(type: string, size = 1000) {
  const f = new File(["x"], "p.jpg", { type });
  Object.defineProperty(f, "size", { value: size });
  return f;
}

function stubDecode(width: number, height: number) {
  const drawImage = vi.fn();
  const canvases: HTMLCanvasElement[] = [];
  vi.stubGlobal("createImageBitmap", vi.fn().mockResolvedValue({ width, height, close: vi.fn() }));
  vi.spyOn(HTMLCanvasElement.prototype, "getContext").mockReturnValue({ drawImage } as unknown as CanvasRenderingContext2D);
  vi.spyOn(HTMLCanvasElement.prototype, "toBlob").mockImplementation(function (this: HTMLCanvasElement, cb, type, quality) {
    canvases.push(this);
    cb(new Blob(["jpg"], { type }));
    toBlobArgs.push([type, quality]);
  });
  return { drawImage, canvases };
}
const toBlobArgs: unknown[][] = [];

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
  toBlobArgs.length = 0;
});

describe("cropRect", () => {
  it("keeps the center of a 3000 x 2000 landscape", () => {
    expect(cropRect(3000, 2000)).toEqual({ sx: 700, sy: 0, sw: 1600, sh: 2000 });
  });
  it("trims top and bottom of a tall image", () => {
    expect(cropRect(1200, 2000)).toEqual({ sx: 0, sy: 250, sw: 1200, sh: 1500 });
  });
  it("leaves an exact 4:5 untouched", () => {
    expect(cropRect(2400, 3000)).toEqual({ sx: 0, sy: 0, sw: 2400, sh: 3000 });
  });
});

describe("preparePhoto", () => {
  it("refuses other types", async () => {
    expect(await preparePhoto(file("image/gif"))).toEqual({ error: "Foto harus JPG, PNG, atau WebP" });
  });

  it("refuses a file over 5 MB before decoding", async () => {
    const decode = vi.fn();
    vi.stubGlobal("createImageBitmap", decode);
    expect(await preparePhoto(file("image/png", MAX_PHOTO_BYTES + 1))).toEqual({ error: "Foto lebih dari 5 MB" });
    expect(decode).not.toHaveBeenCalled();
  });

  it("refuses an image under 1200 x 1500", async () => {
    stubDecode(1199, 2000);
    expect(await preparePhoto(file("image/jpeg"))).toEqual({ error: "Foto minimal 1200 × 1500 px" });
    stubDecode(2000, 1499);
    expect(await preparePhoto(file("image/jpeg"))).toEqual({ error: "Foto minimal 1200 × 1500 px" });
  });

  it("center-crops to a 1200 x 1500 JPEG blob at quality 0.85", async () => {
    const { drawImage, canvases } = stubDecode(3000, 2000);
    const res = await preparePhoto(file("image/png"));
    if ("error" in res) throw new Error(res.error);
    expect(res.blob.type).toBe("image/jpeg");
    expect(canvases[0].width).toBe(1200);
    expect(canvases[0].height).toBe(1500);
    expect(toBlobArgs[0]).toEqual(["image/jpeg", 0.85]);
    expect(drawImage).toHaveBeenCalledWith(expect.anything(), 700, 0, 1600, 2000, 0, 0, 1200, 1500);
  });
});
