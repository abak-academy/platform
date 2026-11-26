export const MAX_PHOTO_BYTES = 5 * 1024 * 1024;
export const PHOTO_WIDTH = 1200;
export const PHOTO_HEIGHT = 1500;
const ACCEPTED = ["image/jpeg", "image/png", "image/webp"];

export interface CropRect {
  sx: number;
  sy: number;
  sw: number;
  sh: number;
}

export function cropRect(w: number, h: number): CropRect {
  const wide = w * PHOTO_HEIGHT > h * PHOTO_WIDTH;
  const sw = wide ? Math.round((h * PHOTO_WIDTH) / PHOTO_HEIGHT) : w;
  const sh = wide ? h : Math.round((w * PHOTO_HEIGHT) / PHOTO_WIDTH);
  return { sx: Math.round((w - sw) / 2), sy: Math.round((h - sh) / 2), sw, sh };
}

export async function preparePhoto(file: File): Promise<{ blob: Blob } | { error: string }> {
  if (!ACCEPTED.includes(file.type)) return { error: "Foto harus JPG, PNG, atau WebP" };
  if (file.size > MAX_PHOTO_BYTES) return { error: "Foto lebih dari 5 MB" };
  let bitmap: ImageBitmap;
  try {
    bitmap = await createImageBitmap(file);
  } catch {
    return { error: "Foto tidak bisa dibaca" };
  }
  if (bitmap.width < PHOTO_WIDTH || bitmap.height < PHOTO_HEIGHT) {
    bitmap.close?.();
    return { error: "Foto minimal 1200 × 1500 px" };
  }
  const { sx, sy, sw, sh } = cropRect(bitmap.width, bitmap.height);
  const canvas = document.createElement("canvas");
  canvas.width = PHOTO_WIDTH;
  canvas.height = PHOTO_HEIGHT;
  canvas.getContext("2d")?.drawImage(bitmap, sx, sy, sw, sh, 0, 0, PHOTO_WIDTH, PHOTO_HEIGHT);
  bitmap.close?.();
  const blob = await new Promise<Blob | null>((resolve) => canvas.toBlob(resolve, "image/jpeg", 0.85));
  return blob ? { blob } : { error: "Foto tidak bisa diproses" };
}
