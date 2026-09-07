import type { PluginRpcClient } from "@get-bb/plugin-sdk";
import type { rpcContract } from "./contract";

export async function uploadPickedImage(
  file: File,
  rpc: PluginRpcClient<typeof rpcContract>,
  options: {
    threadId: string;
    title: string;
    sceneContext: string;
    signal: AbortSignal;
    progress: (percent: number) => void;
  },
) {
  if (file.size < 24 || file.size > 32 * 1024 * 1024)
    throw new Error("Выберите PNG/JPEG размером до 32 MiB.");
  if (!/\.(png|jpe?g)$/i.test(file.name))
    throw new Error("Поддерживаются PNG и JPEG.");
  const uploadId = crypto.randomUUID();
  const bytes = new Uint8Array(await file.arrayBuffer());
  const checksum = Array.from(
    new Uint8Array(await crypto.subtle.digest("SHA-256", bytes)),
    (b) => b.toString(16).padStart(2, "0"),
  ).join("");
  let complete = false;
  try {
    for (
      let offset = 0, index = 0;
      offset < bytes.length;
      offset += 384 * 1024, index++
    ) {
      options.signal.throwIfAborted();
      const chunk = bytes.subarray(offset, offset + 384 * 1024),
        parts: string[] = [];
      for (let i = 0; i < chunk.length; i += 8192)
        parts.push(String.fromCharCode(...chunk.subarray(i, i + 8192)));
      await rpc.call("uploadChunk", {
        threadId: options.threadId,
        uploadId,
        index,
        data: btoa(parts.join("")),
      });
      options.progress(
        Math.round(
          (Math.min(bytes.length, offset + chunk.length) / bytes.length) * 100,
        ),
      );
    }
    options.signal.throwIfAborted();
    const capture = await rpc.call("importUpload", {
      threadId: options.threadId,
      uploadId,
      fileName: file.name,
      totalBytes: bytes.length,
      checksum,
      title: options.title || file.name.slice(0, 200),
      sceneContext: options.sceneContext,
    });
    complete = true;
    return capture;
  } finally {
    if (!complete)
      await rpc
        .call("discardUpload", { threadId: options.threadId, uploadId })
        .catch(() => {});
  }
}
