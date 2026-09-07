import { createHash } from "node:crypto";
import { mkdir, readFile, writeFile, rm } from "node:fs/promises";
import { join } from "node:path";
import { id, digest } from "./contract";
import { ImageStore, sha256 } from "./image-store";

export class Uploads {
  constructor(
    private tempDir: string,
    private store: ImageStore,
  ) {}
  private directory(threadId: string, uploadId: string) {
    id.parse(uploadId);
    const scope = createHash("sha256").update(threadId).digest("hex");
    return join(this.tempDir, "picked-images", scope, uploadId);
  }
  async chunk(threadId: string, uploadId: string, index: number, data: string) {
    if (!Number.isInteger(index) || index < 0 || index > 85)
      throw new Error("Invalid chunk index");
    const bytes = Buffer.from(data, "base64");
    if (
      !bytes.length ||
      bytes.length > 384 * 1024 ||
      bytes.toString("base64") !== data
    )
      throw new Error("Invalid image chunk");
    const dir = this.directory(threadId, uploadId);
    await mkdir(dir, { recursive: true });
    const file = join(dir, String(index));
    try {
      await writeFile(file, bytes, { flag: "wx" });
    } catch (e) {
      if ((e as NodeJS.ErrnoException).code !== "EEXIST") throw e;
      if (!(await readFile(file)).equals(bytes))
        throw new Error("Image chunk changed during upload");
    }
    return { received: bytes.length };
  }
  async finish(
    threadId: string,
    uploadId: string,
    fileName: string,
    totalBytes: number,
    checksum: string,
  ) {
    digest.parse(checksum);
    if (
      !Number.isInteger(totalBytes) ||
      totalBytes < 24 ||
      totalBytes > 32 * 1024 * 1024
    )
      throw new Error("Нужен PNG/JPEG размером до 32 MiB.");
    const dir = this.directory(threadId, uploadId),
      chunks: Buffer[] = [];
    const count = Math.ceil(totalBytes / (384 * 1024));
    for (let i = 0; i < count; i++) {
      const part = await readFile(join(dir, String(i)));
      if (part.length !== Math.min(384 * 1024, totalBytes - i * 384 * 1024))
        throw new Error(
          "Неполная загрузка изображения. Выберите файл ещё раз.",
        );
      chunks.push(part);
    }
    const data = Buffer.concat(chunks);
    if (sha256(data) !== checksum)
      throw new Error(
        "Изображение изменилось при загрузке. Выберите файл ещё раз.",
      );
    const image = await this.store.importBytes(data);
    await this.discard(threadId, uploadId);
    // Browser file selection does not reveal an absolute path. Do not invent one.
    return { image, sourcePath: `Выбранный файл: ${fileName}` };
  }
  async discard(threadId: string, uploadId: string) {
    await rm(this.directory(threadId, uploadId), {
      recursive: true,
      force: true,
    });
    return { discarded: true };
  }
}
