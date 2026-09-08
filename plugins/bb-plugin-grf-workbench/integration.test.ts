import { afterEach, expect, it, vi } from "vitest";
import {
  mkdtemp,
  mkdir,
  readFile,
  rm,
  symlink,
  writeFile,
} from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { PNG } from "pngjs";
import {
  createFakePluginHost,
  makeThreadResponse,
  experimental_scanPublicSdkOnly,
} from "@get-bb/plugin-sdk/testing";
import { experimental_createHostEntryHarness } from "@get-bb/plugin-sdk/testing/host";
import hostEntry from "./host";
import plugin from "./server";
import { type Annotation, type Capture, hostContract } from "./contract";
import { ImageStore } from "./image-store";

const cleanups: (() => Promise<unknown>)[] = [];
afterEach(async () => {
  for (const f of cleanups.splice(0).reverse()) await f();
});
async function fixture(renderOverride?: () => unknown) {
  const root = await mkdtemp(join(tmpdir(), "grf-review-test-"));
  cleanups.push(() => rm(root, { recursive: true, force: true }));
  const workspace = join(root, "workspace"),
    blobs = join(root, "host-data");
  await mkdir(workspace);
  const p = new PNG({ width: 32, height: 24 });
  for (let y = 0; y < 24; y++)
    for (let x = 0; x < 32; x++) {
      const i = (y * 32 + x) * 4;
      p.data.set([x * 7, y * 9, (x + y) * 3, 255], i);
    }
  const source = PNG.sync.write(p);
  await writeFile(join(workspace, "scene.png"), source);
  const worker = experimental_createHostEntryHarness(hostEntry, {
    experimental_paths: { dataDir: blobs, tempDir: root },
  });
  cleanups.push(() => worker.experimental_dispose());
  const env = {
    id: "env-remote",
    hostId: "remote-host",
    path: workspace,
    projectId: "project-1",
    baseBranch: null,
    branchName: null,
    createdAt: 0,
    defaultBranch: null,
    isGitRepo: false,
    isWorktree: false,
    managed: false,
    mergeBaseBranch: null,
    name: null,
    status: "ready" as const,
    updatedAt: 0,
    workspaceProvisionType: "unmanaged" as const,
  };
  const fake = createFakePluginHost({
    pluginId: "grf-workbench",
    dataDir: join(root, "server"),
    experimental_hostEntry: true,
    sdk: {
      threads: {
        get: async () =>
          makeThreadResponse({
            id: "thread-1",
            environmentId: env.id,
            projectId: env.projectId,
          }),
      },
      environments: { get: async () => env },
    },
    experimental_callHostRpc: (call) =>
      call.method === "grfRender" && renderOverride
        ? renderOverride()
        : worker.experimental_call(
            call.method as keyof typeof hostContract,
            call.input as never,
            { signal: call.signal },
          ),
  });
  cleanups.push(() => fake.harness.lifecycle.dispose());
  await plugin(fake.bb);
  return { ...fake, worker, source, workspace, root, blobs };
}
it("routes reads to the scene's host, preserves pixels, persists annotations across reload and delivers images to agents", async () => {
  const f = await fixture();
  const capture = (await f.harness.behavior.callRpc("importImage", {
    threadId: "thread-1",
    path: "scene.png",
    title: "Wall occlusion",
    sceneContext: "Prontera; position not recorded",
  })) as Capture;
  const rect = { x: 5, y: 7, width: 8, height: 6 };
  const a = (await f.harness.behavior.callRpc("annotate", {
    captureId: capture.id,
    imageDigest: capture.image.digest,
    rect,
    comment: "NPC head hidden by wall",
  })) as Annotation;
  await writeFile(join(f.workspace, "scene.png"), "replaced source");
  const reloaded = await f.harness.lifecycle.reload(plugin);
  f.harness = reloaded.harness;
  cleanups.push(() => reloaded.harness.lifecycle.dispose());
  const result = await f.harness.behavior.callAgentTool(
    "grf_review_read",
    { annotationId: a.id },
    { threadId: "thread-1", projectId: "project-1" },
  );
  if (typeof result === "string") throw new Error("Expected images");
  const images = result.content.filter((p) => p.type === "image");
  expect(images).toHaveLength(2);
  expect(Buffer.from(images[0]!.data, "base64")).toEqual(f.source);
  const crop = PNG.sync.read(Buffer.from(images[1]!.data, "base64"));
  const original = PNG.sync.read(f.source);
  expect([crop.width, crop.height]).toEqual([8, 6]);
  for (let y = 0; y < 6; y++)
    for (let x = 0; x < 8; x++)
      expect(crop.data.subarray((y * 8 + x) * 4, (y * 8 + x + 1) * 4)).toEqual(
        original.data.subarray(
          ((y + 7) * 32 + x + 5) * 4,
          ((y + 7) * 32 + x + 6) * 4,
        ),
      );
  expect(
    f.harness.inspection.experimental_hostRpcCalls.every(
      (c) => c.hostId === "remote-host",
    ),
  ).toBe(true);
  await expect(
    f.harness.behavior.callAgentTool(
      "grf_review_read",
      { annotationId: a.id },
      { projectId: "other-project" },
    ),
  ).rejects.toThrow();
  const exported = await f.harness.behavior.runCli(
    ["export", a.id, ".", "--json"],
    { threadId: "thread-1" },
  );
  expect(exported.exitCode).toBe(0);
  const files = JSON.parse(exported.stdout!);
  expect(await readFile(files.original)).toEqual(f.source);
  expect(
    JSON.parse(await readFile(files.manifest, "utf8")).annotation.rect,
  ).toEqual(rect);
  const resolved = (await f.harness.behavior.callRpc("resolve", {
    annotationId: a.id,
    revision: 0,
    resolved: true,
  })) as Annotation;
  expect(resolved.revision).toBe(1);
  await expect(
    f.harness.behavior.callRpc("resolve", {
      annotationId: a.id,
      revision: 0,
      resolved: false,
    }),
  ).rejects.toThrow("другой вкладке");
  await expect(
    f.harness.behavior.callRpc("annotate", {
      captureId: capture.id,
      imageDigest: "f".repeat(64),
      rect,
      comment: "stale",
    }),
  ).rejects.toThrow("Версия");
});
it("rejects escaping/symlink paths and out-of-bounds rectangles", async () => {
  const f = await fixture();
  await writeFile(join(f.root, "outside.png"), f.source);
  await symlink(join(f.root, "outside.png"), join(f.workspace, "link.png"));
  for (const path of ["../outside.png", "link.png"])
    await expect(
      f.harness.behavior.callRpc("importImage", {
        threadId: "thread-1",
        path,
        title: "bad",
        sceneContext: "",
      }),
    ).rejects.toThrow("внутри");
  const c = (await f.harness.behavior.callRpc("importImage", {
    threadId: "thread-1",
    path: "scene.png",
    title: "test",
    sceneContext: "",
  })) as Capture;
  await expect(
    f.harness.behavior.callRpc("annotate", {
      captureId: c.id,
      imageDigest: c.image.digest,
      rect: { x: 31, y: 0, width: 2, height: 2 },
      comment: "bad",
    }),
  ).rejects.toThrow("границы");
});
it("chunks larger images without base64 corruption", async () => {
  const f = await fixture();
  const p = new PNG({ width: 512, height: 512 });
  let seed = 42;
  for (let i = 0; i < p.data.length; i++) {
    seed = (Math.imul(seed, 1664525) + 1013904223) >>> 0;
    p.data[i] = seed >>> 24;
  }
  const source = PNG.sync.write(p);
  expect(source.length).toBeGreaterThan(384 * 1024);
  await writeFile(join(f.workspace, "large.png"), source);
  const store = new ImageStore(f.blobs),
    imported = await store.importImage(f.workspace, "large.png");
  const parts: string[] = [];
  let offset = 0;
  for (;;) {
    const c = await store.image(imported.image.digest, offset);
    parts.push(c.data);
    if (c.done) break;
    offset = c.nextOffset;
  }
  expect(Buffer.from(parts.join(""), "base64")).toEqual(source);
});
it("uses only public plugin SDK imports", async () => {
  const scan = await experimental_scanPublicSdkOnly(process.cwd(), {
    allow: [
      /^pngjs$/,
      /^jpeg-js$/,
      /^vitest$/,
      /^@testing-library\/react$/,
      /^react$/,
      /^@radix-ui\//,
      /^@hugeicons\//,
      /^class-variance-authority$/,
      /^clsx$/,
      /^tailwind-merge$/,
      /^sonner$/,
      /^vaul$/,
    ],
  });
  expect(scan.violations).toEqual([]);
  expect(scan.privateDependencies).toEqual([]);
});

it("imports picked file bytes on the target host and validates chunks, scope and checksum", async () => {
  const f = await fixture();
  const sourceImage = new PNG({ width: 512, height: 512 });
  let seed = 13;
  for (let i = 0; i < sourceImage.data.length; i++) {
    seed = (Math.imul(seed, 1664525) + 1013904223) >>> 0;
    sourceImage.data[i] = seed >>> 24;
  }
  const source = PNG.sync.write(sourceImage);
  const { createHash, randomUUID } = await import("node:crypto");
  const uploadId = randomUUID(),
    scope = { threadId: "thread-1", uploadId };
  for (
    let offset = 0, index = 0;
    offset < source.length;
    offset += 384 * 1024, index++
  ) {
    const input = {
      ...scope,
      index,
      data: source.subarray(offset, offset + 384 * 1024).toString("base64"),
    };
    await f.harness.behavior.callRpc("uploadChunk", input);
    if (index === 0) await f.harness.behavior.callRpc("uploadChunk", input); // Safe retransmission.
  }
  await expect(
    f.harness.behavior.callRpc("uploadChunk", {
      ...scope,
      index: 0,
      data: "YWJj",
    }),
  ).rejects.toThrow("changed");
  const complete = {
    ...scope,
    fileName: "Снимок с рабочего стола.png",
    totalBytes: source.length,
    checksum: createHash("sha256").update(source).digest("hex"),
    title: "Picked screenshot",
    sceneContext: "Wall occlusion",
  };
  await expect(
    f.harness.behavior.callRpc("importUpload", {
      ...complete,
      threadId: "another-thread",
    }),
  ).rejects.toThrow();
  await expect(
    f.harness.behavior.callRpc("importUpload", {
      ...complete,
      checksum: "f".repeat(64),
    }),
  ).rejects.toThrow("изменилось");
  const c = (await f.harness.behavior.callRpc(
    "importUpload",
    complete,
  )) as Capture;
  expect(c.sourcePath).toBe("Выбранный файл: Снимок с рабочего стола.png");
  expect(c.hostId).toBe("remote-host");
  expect(await new ImageStore(f.blobs).read(c.image.digest)).toEqual(source);
  await expect(
    f.harness.behavior.callRpc("importUpload", complete),
  ).rejects.toThrow(); // Temporary parts removed.
  const badScope = { threadId: "thread-1", uploadId: randomUUID() };
  await f.harness.behavior.callRpc("uploadChunk", {
    ...badScope,
    index: 0,
    data: Buffer.alloc(30).toString("base64"),
  });
  await expect(
    f.harness.behavior.callRpc("importUpload", {
      ...complete,
      ...badScope,
      totalBytes: 30,
      checksum: createHash("sha256").update(Buffer.alloc(30)).digest("hex"),
    }),
  ).rejects.toThrow("PNG");
  await f.harness.behavior.callRpc("discardUpload", badScope);
});

it("freezes a GRF atlas frame with durable resource provenance for annotation", async () => {
  let preview: unknown;
  const f = await fixture(() => preview);
  const atlas = new PNG({ width: 4, height: 2 });
  for (let i = 0; i < 8; i++)
    atlas.data.set(i % 4 < 2 ? [255, 0, 0, 255] : [0, 0, 255, 255], i * 4);
  const image = await new ImageStore(f.blobs).importBytes(
    PNG.sync.write(atlas),
  );
  const entry = {
    id: "c".repeat(64),
    path: "data/sprite/npc/rocker.act",
    type: "act",
    archive: 0,
    source: "/archives/data.grf",
    bytes: 500,
    variants: [0],
  };
  preview = {
    info: {
      entry,
      version: "2.5",
      imageCount: 0,
      actions: [{ index: 0, name: "Idle", frames: 2, intervalMs: 100 }],
      pair: null,
      warnings: [],
    },
    fingerprint: "d".repeat(64),
    dependencies: [
      { path: entry.path, source: entry.source, sha256: "e".repeat(64) },
    ],
    renderer: "midgard-ro/sprite.CompositeSprites",
    action: 0,
    firstFrame: 3,
    frameCount: 2,
    width: 2,
    height: 2,
    columns: 2,
    originX: 1,
    originY: 2,
    intervalMs: 100,
    colorKey: true,
    image,
    rendererVersion: "f".repeat(64),
  };
  const p = (await f.harness.behavior.callRpc("grfRender", {
    threadId: "thread-1",
    id: entry.id,
    archive: -1,
    action: 0,
    frame: 3,
    animated: true,
    colorKey: true,
  })) as { previewId: string };
  const c = (await f.harness.behavior.callRpc("grfCapture", {
    previewId: p.previewId,
    frame: 1,
  })) as Capture;
  expect(c.resource).toMatchObject({
    entry,
    action: 0,
    frame: 4,
    originX: 1,
    originY: 2,
    rendererVersion: "f".repeat(64),
  });
  const frozen = PNG.sync.read(
    await new ImageStore(f.blobs).read(c.image.digest),
  );
  expect([frozen.width, frozen.height]).toEqual([2, 2]);
  expect([...frozen.data]).toEqual(
    Array.from({ length: 4 }, () => [0, 0, 255, 255]).flat(),
  );
  const a = (await f.harness.behavior.callRpc("annotate", {
    captureId: c.id,
    imageDigest: c.image.digest,
    rect: { x: 0, y: 0, width: 1, height: 1 },
    comment: "Check this ACT frame",
  })) as Annotation;
  const result = await f.harness.behavior.callAgentTool(
    "grf_review_read",
    { annotationId: a.id },
    { projectId: "project-1" },
  );
  if (typeof result === "string") throw new Error("Expected structured images");
  expect(result.content[0]).toMatchObject({
    type: "text",
    text: expect.stringContaining('"frame": 4'),
  });
});

it("edits, removes and restores regions with revision checks and preserved image data", async () => {
  const f = await fixture();
  const c = (await f.harness.behavior.callRpc("importImage", {
    threadId: "thread-1",
    path: "scene.png",
    title: "Regions",
    sceneContext: "",
  })) as Capture;
  const a = (await f.harness.behavior.callRpc("annotate", {
    captureId: c.id,
    imageDigest: c.image.digest,
    rect: { x: 2, y: 3, width: 5, height: 4 },
    comment: "Original comment",
  })) as Annotation;
  const edited = (await f.harness.behavior.callRpc("updateAnnotation", {
    annotationId: a.id,
    revision: 0,
    comment: "Checked this region",
    resolved: true,
  })) as Annotation;
  expect(edited).toMatchObject({
    revision: 1,
    comment: "Checked this region",
    resolved: true,
    rect: a.rect,
  });
  await expect(
    f.harness.behavior.callRpc("deleteAnnotation", {
      annotationId: a.id,
      revision: 0,
    }),
  ).rejects.toThrow("другой вкладке");
  const deleted = (await f.harness.behavior.callRpc("deleteAnnotation", {
    annotationId: a.id,
    revision: 1,
  })) as Annotation;
  expect(deleted.deletedAt).toBeTruthy();
  expect(deleted.revision).toBe(2);
  expect(
    await f.harness.behavior.callRpc("get", { captureId: c.id }),
  ).toMatchObject({ annotations: [] });
  await expect(
    f.harness.behavior.callAgentTool(
      "grf_review_read",
      { annotationId: a.id },
      { projectId: "project-1" },
    ),
  ).rejects.toThrow("удалена");
  await expect(
    f.harness.behavior.callRpc("updateAnnotation", {
      annotationId: a.id,
      revision: 2,
      comment: "Do not resurrect",
    }),
  ).rejects.toThrow("удалена");
  await expect(
    f.harness.behavior.callRpc("restoreAnnotation", {
      annotationId: a.id,
      revision: 1,
    }),
  ).rejects.toThrow("другой вкладке");
  const restored = (await f.harness.behavior.callRpc("restoreAnnotation", {
    annotationId: a.id,
    revision: 2,
  })) as Annotation;
  expect(restored).toMatchObject({
    id: a.id,
    deletedAt: null,
    revision: 3,
    comment: edited.comment,
    resolved: true,
    rect: a.rect,
  });
  const r = await f.harness.lifecycle.reload(plugin);
  f.harness = r.harness;
  cleanups.push(() => r.harness.lifecycle.dispose());
  expect(
    await f.harness.behavior.callRpc("get", { captureId: c.id }),
  ).toMatchObject({ annotations: [restored] });
  await f.harness.behavior.callRpc("deleteAnnotation", {
    annotationId: a.id,
    revision: 3,
  });
  expect(await new ImageStore(f.blobs).read(c.image.digest)).toEqual(f.source);
  expect(await new ImageStore(f.blobs).read(a.crop.digest)).toBeInstanceOf(
    Buffer,
  );
});

it("updates region geometry and crop together, preserving the comment and invalidating the checked state", async () => {
  const f = await fixture();
  const c = (await f.harness.behavior.callRpc("importImage", {
    threadId: "thread-1",
    path: "scene.png",
    title: "Geometry",
    sceneContext: "",
  })) as Capture;
  const a = (await f.harness.behavior.callRpc("annotate", {
    captureId: c.id,
    imageDigest: c.image.digest,
    rect: { x: 2, y: 3, width: 5, height: 4 },
    comment: "Keep this comment",
  })) as Annotation;
  await f.harness.behavior.callRpc("resolve", {
    annotationId: a.id,
    revision: 0,
    resolved: true,
  });
  const rect = { x: 4, y: 5, width: 6, height: 7 };
  const changed = (await f.harness.behavior.callRpc("transformAnnotation", {
    annotationId: a.id,
    revision: 1,
    imageDigest: c.image.digest,
    rect,
  })) as Annotation;
  expect(changed).toMatchObject({
    rect,
    revision: 2,
    resolved: false,
    comment: a.comment,
    imageDigest: c.image.digest,
  });
  const crop = PNG.sync.read(
    await new ImageStore(f.blobs).read(changed.crop.digest),
  );
  const source = PNG.sync.read(f.source);
  expect([crop.width, crop.height]).toEqual([6, 7]);
  for (let y = 0; y < 7; y++)
    for (let x = 0; x < 6; x++)
      expect(crop.data.subarray((y * 6 + x) * 4, (y * 6 + x + 1) * 4)).toEqual(
        source.data.subarray(
          ((y + 5) * 32 + x + 4) * 4,
          ((y + 5) * 32 + x + 5) * 4,
        ),
      );
  await expect(
    f.harness.behavior.callRpc("transformAnnotation", {
      annotationId: a.id,
      revision: 1,
      imageDigest: c.image.digest,
      rect: a.rect,
    }),
  ).rejects.toThrow("другой вкладке");
  await expect(
    f.harness.behavior.callRpc("transformAnnotation", {
      annotationId: a.id,
      revision: 2,
      imageDigest: c.image.digest,
      rect: { x: 31, y: 0, width: 2, height: 2 },
    }),
  ).rejects.toThrow("границы");
  await expect(
    f.harness.behavior.callRpc("transformAnnotation", {
      annotationId: a.id,
      revision: 2,
      imageDigest: "f".repeat(64),
      rect: a.rect,
    }),
  ).rejects.toThrow("Версия");
  const unchanged = (await f.harness.behavior.callRpc("transformAnnotation", {
    annotationId: a.id,
    revision: 2,
    imageDigest: c.image.digest,
    rect,
  })) as Annotation;
  expect(unchanged.revision).toBe(2);
});

it("deletes a capture with all annotations while preserving another capture of the same image", async () => {
  const f = await fixture();
  const input = {
    threadId: "thread-1",
    path: "scene.png",
    title: "Disposable review",
    sceneContext: "",
  };
  const capture = (await f.harness.behavior.callRpc(
    "importImage",
    input,
  )) as Capture;
  const other = (await f.harness.behavior.callRpc("importImage", {
    ...input,
    title: "Keep this review",
  })) as Capture;
  const a = (await f.harness.behavior.callRpc("annotate", {
    captureId: capture.id,
    imageDigest: capture.image.digest,
    rect: { x: 0, y: 0, width: 5, height: 5 },
    comment: "Remove with capture",
  })) as Annotation;
  await f.harness.behavior.callRpc("deleteAnnotation", {
    annotationId: a.id,
    revision: a.revision,
  });
  await expect(
    f.harness.behavior.callRpc("deleteCapture", {
      captureId: capture.id,
      imageDigest: "0".repeat(64),
    }),
  ).rejects.toThrow("Версия снимка");
  const result = await f.harness.behavior.callRpc("deleteCapture", {
    captureId: capture.id,
    imageDigest: capture.image.digest,
  });
  expect(result).toMatchObject({
    captureId: capture.id,
    deletedAnnotations: 1,
  });
  expect(
    await f.harness.behavior.callRpc("list", { threadId: "thread-1" }),
  ).toEqual([other]);
  await expect(
    f.harness.behavior.callRpc("get", { captureId: capture.id }),
  ).rejects.toThrow("Снимок не найден");
  await expect(
    f.harness.behavior.callRpc("restoreAnnotation", {
      annotationId: a.id,
      revision: 1,
    }),
  ).rejects.toThrow("Замечание не найдено");
  const image = (await f.harness.behavior.callRpc("image", {
    captureId: other.id,
    offset: 0,
  })) as { data: string };
  expect(image.data.length).toBeGreaterThan(0);
  const reload = await f.harness.lifecycle.reload(plugin);
  f.harness = reload.harness;
  cleanups.push(() => reload.harness.lifecycle.dispose());
  await expect(
    f.harness.behavior.callRpc("get", { captureId: capture.id }),
  ).rejects.toThrow("Снимок не найден");
});

it("does not create an orphan annotation when capture deletion races with a host crop", async () => {
  const f = await fixture();
  const capture = (await f.harness.behavior.callRpc("importImage", {
    threadId: "thread-1",
    path: "scene.png",
    title: "Race test",
    sceneContext: "",
  })) as Capture;
  let release!: () => void, entered!: () => void;
  const gate = new Promise<void>((r) => {
      release = r;
    }),
    ready = new Promise<void>((r) => {
      entered = r;
    });
  const original = ImageStore.prototype.cropImage;
  const spy = vi
    .spyOn(ImageStore.prototype, "cropImage")
    .mockImplementation(async function (
      this: ImageStore,
      ...args: Parameters<ImageStore["cropImage"]>
    ) {
      entered();
      await gate;
      return original.apply(this, args);
    });
  try {
    const saving = f.harness.behavior.callRpc("annotate", {
      captureId: capture.id,
      imageDigest: capture.image.digest,
      rect: { x: 0, y: 0, width: 5, height: 5 },
      comment: "Pending crop",
    });
    const rejected = expect(saving).rejects.toThrow("Снимок не найден");
    await ready;
    await f.harness.behavior.callRpc("deleteCapture", {
      captureId: capture.id,
      imageDigest: capture.image.digest,
    });
    release();
    await rejected;
    expect(
      await f.harness.behavior.callRpc("list", { threadId: "thread-1" }),
    ).toEqual([]);
  } finally {
    release();
    spy.mockRestore();
  }
});
