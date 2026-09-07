import { createHash, randomUUID } from "node:crypto";
import {
  mkdir,
  open,
  readFile,
  realpath,
  rename,
  writeFile,
} from "node:fs/promises";
import { dirname, isAbsolute, join, relative, resolve, sep } from "node:path";
import { PNG } from "pngjs";
import jpeg from "jpeg-js";
import type { Bundle, Rect } from "./contract";
import { digest as digestSchema } from "./contract";
import { assertRect } from "./geometry";

const MAX_BYTES = 32 * 1024 * 1024;
const MAX_PIXELS = 16 * 1024 * 1024;
export function sha256(data: Buffer) {
  return createHash("sha256").update(data).digest("hex");
}
function inside(root: string, path: string) {
  const r = relative(root, path);
  return r !== ".." && !r.startsWith(`..${sep}`) && !isAbsolute(r);
}
export async function confinedPath(root: string, path: string) {
  const actualRoot = await realpath(root);
  const actualPath = await realpath(resolve(actualRoot, path));
  if (!inside(actualRoot, actualPath))
    throw new Error(
      "Выберите изображение внутри рабочего каталога проекта (ссылки наружу не поддерживаются).",
    );
  return actualPath;
}
function dimensionsOK(width: number, height: number) {
  if (width < 1 || height < 1 || width * height > MAX_PIXELS)
    throw new Error("Изображение должно содержать не более 16 мегапикселей.");
}
export function decode(data: Buffer) {
  if (data.length < 24 || data.length > MAX_BYTES)
    throw new Error("Нужен PNG или JPEG размером до 32 MiB.");
  if (
    data.subarray(0, 8).equals(Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]))
  ) {
    dimensionsOK(data.readUInt32BE(16), data.readUInt32BE(20));
    const png = PNG.sync.read(data, { checkCRC: true });
    return { ...png, mimeType: "image/png" as const };
  }
  if (data[0] === 255 && data[1] === 216) {
    const img = jpeg.decode(data, {
      useTArray: true,
      maxResolutionInMP: 16,
      maxMemoryUsageInMB: 160,
      tolerantDecoding: false,
    });
    dimensionsOK(img.width, img.height);
    return { ...img, mimeType: "image/jpeg" as const };
  }
  throw new Error(
    "Поддерживаются PNG и JPEG. Изображение не удалось прочитать.",
  );
}
export class ImageStore {
  constructor(private directory: string) {}
  private blobPath(hash: string) {
    digestSchema.parse(hash);
    return join(this.directory, "images", hash);
  }
  async read(hash: string) {
    const data = await readFile(this.blobPath(hash));
    if (sha256(data) !== hash)
      throw new Error("Контрольная сумма сохранённого изображения изменилась.");
    return data;
  }
  private async put(data: Buffer) {
    const img = decode(data),
      hash = sha256(data),
      path = this.blobPath(hash);
    await mkdir(dirname(path), { recursive: true });
    const temporary = `${path}.${randomUUID()}.tmp`;
    await writeFile(temporary, data, { flag: "wx" });
    await rename(temporary, path);
    return {
      digest: hash,
      width: img.width,
      height: img.height,
      mimeType: img.mimeType,
      bytes: data.length,
    };
  }
  async importBytes(data: Buffer) {
    return this.put(data);
  }
  async importImage(root: string, path: string) {
    const sourcePath = await confinedPath(root, path);
    const handle = await open(sourcePath, "r");
    let data: Buffer;
    try {
      const stat = await handle.stat();
      if (!stat.isFile() || stat.size > MAX_BYTES)
        throw new Error("Нужен файл изображения размером до 32 MiB.");
      // Bounded read also handles a file growing after stat (e.g. latest.png).
      const buffer = Buffer.alloc(MAX_BYTES + 1);
      let size = 0;
      while (size < buffer.length) {
        const r = await handle.read(buffer, size, buffer.length - size, null);
        if (!r.bytesRead) break;
        size += r.bytesRead;
      }
      data = buffer.subarray(0, size);
    } finally {
      await handle.close();
    }
    return { image: await this.put(data), sourcePath };
  }
  async image(hash: string, offset: number) {
    const data = await this.read(hash);
    if (
      !Number.isSafeInteger(offset) ||
      offset < 0 ||
      offset > data.length ||
      offset % (384 * 1024) !== 0
    )
      throw new Error("Invalid image chunk offset");
    const nextOffset = Math.min(data.length, offset + 384 * 1024);
    const mimeType =
      data[0] === 137 ? ("image/png" as const) : ("image/jpeg" as const);
    return {
      mimeType,
      data: data.subarray(offset, nextOffset).toString("base64"),
      nextOffset,
      done: nextOffset === data.length,
    };
  }
  async agentImage(hash: string) {
    const data = await this.read(hash),
      src = decode(data);
    if (data.length <= 2 * 1024 * 1024)
      return {
        image: { mimeType: src.mimeType, data: data.toString("base64") },
        width: src.width,
        height: src.height,
      };
    // The exact original remains available in UI/export. Bound model images;
    // report their dimensions so source-pixel coordinates stay unambiguous.
    let scale = Math.min(1, 1600 / Math.max(src.width, src.height));
    for (;;) {
      const width = Math.max(1, Math.floor(src.width * scale)),
        height = Math.max(1, Math.floor(src.height * scale));
      const dst = new PNG({ width, height });
      for (let y = 0; y < height; y++)
        for (let x = 0; x < width; x++) {
          const i =
            (Math.min(src.height - 1, Math.floor((y * src.height) / height)) *
              src.width +
              Math.min(src.width - 1, Math.floor((x * src.width) / width))) *
            4;
          dst.data.set(src.data.subarray(i, i + 4), (y * width + x) * 4);
        }
      const png = PNG.sync.write(dst);
      if (png.length <= 2 * 1024 * 1024)
        return {
          image: {
            mimeType: "image/png" as const,
            data: png.toString("base64"),
          },
          width,
          height,
        };
      scale *= 0.7;
    }
  }
  async cropImage(hash: string, rect: Rect) {
    const source = decode(await this.read(hash));
    assertRect(rect, source.width, source.height);
    const crop = new PNG({ width: rect.width, height: rect.height });
    for (let row = 0; row < rect.height; row++) {
      const start = ((rect.y + row) * source.width + rect.x) * 4;
      crop.data.set(
        source.data.subarray(start, start + rect.width * 4),
        row * rect.width * 4,
      );
    }
    return this.put(PNG.sync.write(crop));
  }
  async exportAnnotation(root: string, directory: string, bundle: Bundle) {
    // Export only into an existing real workspace directory; never follow a
    // user-supplied filename or replace existing artifacts.
    const parent = await confinedPath(root, directory);
    const dir = join(
      parent,
      `grf-review-${bundle.annotation.id}-${randomUUID().slice(0, 8)}`,
    );
    await mkdir(dir);
    const original = join(
      dir,
      bundle.capture.image.mimeType === "image/png"
        ? "original.png"
        : "original.jpg",
    );
    const crop = join(dir, "region.png"),
      manifest = join(dir, "review.json");
    await writeFile(original, await this.read(bundle.capture.image.digest), {
      flag: "wx",
    });
    await writeFile(crop, await this.read(bundle.annotation.crop.digest), {
      flag: "wx",
    });
    await writeFile(
      manifest,
      JSON.stringify(
        {
          schemaVersion: 1,
          ...bundle,
          files: { original: original.split(sep).pop(), crop: "region.png" },
        },
        null,
        2,
      ) + "\n",
      { flag: "wx" },
    );
    return { manifest, original, crop };
  }
}
