import { randomUUID } from "node:crypto";
import type { BbPluginApi, PluginAgentToolResult } from "@get-bb/plugin-sdk";
import { z } from "zod";
import {
  grfPreviewSchema,
  grfRenderInput,
  digest,
  annotationSchema,
  captureSchema,
  hostContract,
  id,
  rpcContract,
  type Annotation,
  type Bundle,
  type Capture,
} from "./contract";
import { assertRect } from "./geometry";
export { rpcContract } from "./contract";

export function describe(bundle: Bundle) {
  const { capture: c, annotation: a } = bundle;
  return JSON.stringify(
    {
      schemaVersion: 1,
      annotationId: a.id,
      captureId: c.id,
      title: c.title,
      comment: a.comment,
      ...(c.resource ? { resource: c.resource } : {}),
      sceneContext: c.sceneContext,
      rect: a.rect,
      coordinates:
        "Original image pixels, top-left origin; crop starts at rect.x, rect.y",
      sourceSize: { width: c.image.width, height: c.image.height },
      sourcePath: c.sourcePath,
      imageDigest: c.image.digest,
      cropDigest: a.crop.digest,
      createdAt: a.createdAt,
      resolved: a.resolved,
      reference:
        "Current midgard-ro game client. A screenshot is evidence; do not infer depth/occluder identity from it alone.",
    },
    null,
    2,
  );
}
export default function plugin(bb: BbPluginApi) {
  const host = bb.hosts.experimental_client({ contract: hostContract });
  const db = bb.storage.database();
  bb.storage.migrate(db, [
    "CREATE TABLE captures (id TEXT PRIMARY KEY, thread_id TEXT NOT NULL, project_id TEXT NOT NULL, created_at TEXT NOT NULL, payload TEXT NOT NULL)",
    "CREATE TABLE annotations (id TEXT PRIMARY KEY, capture_id TEXT NOT NULL REFERENCES captures(id), created_at TEXT NOT NULL, payload TEXT NOT NULL)",
    "CREATE INDEX captures_thread ON captures(thread_id, created_at)",
    "CREATE INDEX annotations_capture ON annotations(capture_id, created_at)",
    "CREATE TABLE grf_previews (id TEXT PRIMARY KEY, payload TEXT NOT NULL)",
  ]);
  const getCapture = (captureId: string): Capture => {
    id.parse(captureId);
    const row = db
      .prepare("SELECT payload FROM captures WHERE id=?")
      .get(captureId) as { payload: string } | undefined;
    if (!row) throw new Error("Снимок не найден.");
    return captureSchema.parse(JSON.parse(row.payload));
  };
  const getAnnotation = (
    annotationId: string,
    includeDeleted = false,
  ): Annotation => {
    id.parse(annotationId);
    const row = db
      .prepare("SELECT payload FROM annotations WHERE id=?")
      .get(annotationId) as { payload: string } | undefined;
    if (!row) throw new Error("Замечание не найдено.");
    const a = annotationSchema.parse(JSON.parse(row.payload));
    if (a.deletedAt && !includeDeleted) throw new Error("Область удалена.");
    return a;
  };
  function changeAnnotation(
    annotationId: string,
    revision: number,
    change: (a: Annotation) => Annotation,
    includeDeleted = false,
  ) {
    const next = db.transaction(() => {
      const a = getAnnotation(annotationId, includeDeleted);
      if (a.revision !== revision)
        throw new Error(
          "Область изменена в другой вкладке. Выберите её заново.",
        );
      const next = { ...change(a), revision: a.revision + 1 };
      db.prepare("UPDATE annotations SET payload=? WHERE id=?").run(
        JSON.stringify(next),
        annotationId,
      );
      return next;
    })();
    bb.realtime.publish("review-changed", { captureId: next.captureId });
    return next;
  }
  const bundleFor = (annotationId: string): Bundle => {
    const annotation = getAnnotation(annotationId);
    return { annotation, capture: getCapture(annotation.captureId) };
  };
  const annotationsFor = (captureId: string) =>
    (
      db
        .prepare(
          "SELECT payload FROM annotations WHERE capture_id=? AND json_extract(payload, '$.deletedAt') IS NULL ORDER BY created_at, id",
        )
        .all(captureId) as { payload: string }[]
    ).map((r) => annotationSchema.parse(JSON.parse(r.payload)));
  const list = (threadId: string) =>
    (
      db
        .prepare(
          "SELECT payload FROM captures WHERE thread_id=? ORDER BY created_at DESC, id LIMIT 100",
        )
        .all(threadId) as { payload: string }[]
    ).map((r) => captureSchema.parse(JSON.parse(r.payload)));
  const listSummary = (threadId: string) =>
    list(threadId)
      .slice(0, 20)
      .map((c) => {
        const all = annotationsFor(c.id);
        return {
          captureId: c.id,
          title: c.title,
          createdAt: c.createdAt,
          annotationCount: all.length,
          annotations: all.slice(-20).map((a) => ({
            id: a.id,
            comment: a.comment.slice(0, 200),
            rect: a.rect,
            resolved: a.resolved,
          })),
        };
      });
  async function location(threadId: string) {
    const thread = await bb.sdk.threads.get({ threadId });
    if (!thread.environmentId)
      throw new Error("У обсуждения нет рабочего окружения.");
    const env = await bb.sdk.environments.get({
      environmentId: thread.environmentId,
    });
    if (!env.path) throw new Error("У окружения нет рабочего каталога.");
    return { root: env.path, hostId: env.hostId, projectId: env.projectId };
  }
  async function importImage(
    input: z.infer<typeof rpcContract.importImage.input>,
    signal?: AbortSignal,
  ) {
    const p = rpcContract.importImage.input.parse(input),
      loc = await location(p.threadId);
    const result = await host.call(
      "importImage",
      { root: loc.root, path: p.path },
      { hostId: loc.hostId, signal },
    );
    return recordCapture(p, loc, result);
  }
  function recordCapture(
    p: {
      threadId: string;
      title: string;
      sceneContext: string;
      resource?: Capture["resource"];
    },
    loc: { projectId: string; hostId: string },
    result: { sourcePath: string; image: Capture["image"] },
  ) {
    const capture: Capture = {
      id: randomUUID(),
      threadId: p.threadId,
      projectId: loc.projectId,
      hostId: loc.hostId,
      title: p.title,
      sceneContext: p.sceneContext,
      sourcePath: result.sourcePath,
      image: result.image,
      ...(p.resource ? { resource: p.resource } : {}),
      createdAt: new Date().toISOString(),
    };
    db.prepare("INSERT INTO captures VALUES (?, ?, ?, ?, ?)").run(
      capture.id,
      capture.threadId,
      capture.projectId,
      capture.createdAt,
      JSON.stringify(capture),
    );
    bb.realtime.publish("review-changed", { threadId: p.threadId });
    return capture;
  }
  async function image(
    captureId: string,
    offset: number,
    annotationId?: string,
    signal?: AbortSignal,
  ) {
    const c = getCapture(captureId),
      a = annotationId ? getAnnotation(annotationId) : null;
    if (a && a.captureId !== c.id)
      throw new Error("Замечание относится к другому снимку.");
    return host.call(
      "readImage",
      { digest: a?.crop.digest ?? c.image.digest, offset },
      { hostId: c.hostId, signal },
    );
  }
  async function searchAssets(
    input: z.infer<typeof rpcContract.grfSearch.input>,
    signal?: AbortSignal,
  ) {
    const { threadId, ...request } = rpcContract.grfSearch.input.parse(input),
      loc = await location(threadId);
    return host.call(
      "grfSearch",
      { root: loc.root, ...request },
      { hostId: loc.hostId, signal },
    );
  }
  async function inspectAsset(
    input: z.infer<typeof rpcContract.grfInspect.input>,
    signal?: AbortSignal,
  ) {
    const { threadId, ...request } = rpcContract.grfInspect.input.parse(input),
      loc = await location(threadId);
    return host.call(
      "grfInspect",
      { root: loc.root, ...request },
      { hostId: loc.hostId, signal },
    );
  }
  async function renderAsset(
    input: z.infer<typeof rpcContract.grfRender.input>,
    signal?: AbortSignal,
  ) {
    const { threadId, ...request } = rpcContract.grfRender.input.parse(input),
      loc = await location(threadId);
    const preview = await host.call(
      "grfRender",
      { root: loc.root, ...request },
      { hostId: loc.hostId, signal },
    );
    const previewId = randomUUID();
    db.prepare("INSERT INTO grf_previews VALUES (?, ?)").run(
      previewId,
      JSON.stringify({
        preview,
        threadId,
        hostId: loc.hostId,
        projectId: loc.projectId,
      }),
    );
    return { ...preview, previewId };
  }
  function storedPreview(previewId: string) {
    id.parse(previewId);
    const row = db
      .prepare("SELECT payload FROM grf_previews WHERE id=?")
      .get(previewId) as { payload: string } | undefined;
    if (!row) throw new Error("GRF preview not found");
    return z
      .object({
        preview: grfPreviewSchema,
        threadId: z.string(),
        hostId: z.string(),
        projectId: z.string(),
      })
      .parse(JSON.parse(row.payload));
  }
  bb.rpc.register(rpcContract, {
    grfSearch: (input) => searchAssets(input),
    grfInspect: (input) => inspectAsset(input),
    grfRender: (input) => renderAsset(input),
    grfImage: ({ previewId, offset }) => {
      const p = storedPreview(previewId);
      return host.call(
        "readImage",
        { digest: p.preview.image.digest, offset },
        { hostId: p.hostId },
      );
    },
    grfCapture: async ({ previewId, frame }) => {
      const saved = storedPreview(previewId),
        p = saved.preview;
      if (frame >= p.frameCount) throw new Error("Frame out of range");
      const image = await host.call(
        "cropImage",
        {
          digest: p.image.digest,
          rect: {
            x: (frame % p.columns) * p.width,
            y: Math.floor(frame / p.columns) * p.height,
            width: p.width,
            height: p.height,
          },
        },
        { hostId: saved.hostId },
      );
      const resource = {
        entry: p.info.entry,
        action: p.action,
        frame: p.firstFrame + frame,
        originX: p.originX,
        originY: p.originY,
        intervalMs: p.intervalMs,
        renderer: p.renderer,
        rendererVersion: p.rendererVersion,
        fingerprint: p.fingerprint,
        dependencies: p.dependencies,
        colorKey: p.colorKey,
      };
      return recordCapture(
        {
          threadId: saved.threadId,
          title: `${p.info.entry.path.split("/").pop()} · action ${p.action} · frame ${p.firstFrame + frame}`,
          sceneContext: p.info.warnings.join("\n"),
          resource,
        },
        saved,
        { sourcePath: `${p.info.entry.source} :: ${p.info.entry.path}`, image },
      );
    },
    uploadChunk: async (input) => {
      const loc = await location(input.threadId);
      return host.call("uploadChunk", input, { hostId: loc.hostId });
    },
    importUpload: async ({ title, sceneContext, ...input }) => {
      const loc = await location(input.threadId);
      const result = await host.call("finishUpload", input, {
        hostId: loc.hostId,
      });
      return recordCapture(
        { threadId: input.threadId, title, sceneContext },
        loc,
        result,
      );
    },
    discardUpload: async (input) => {
      const loc = await location(input.threadId);
      return host.call("discardUpload", input, { hostId: loc.hostId });
    },
    list: ({ threadId }) => list(threadId),
    importImage: (input) => importImage(input),
    get: ({ captureId }) => ({
      capture: getCapture(captureId),
      annotations: annotationsFor(captureId),
    }),
    annotation: ({ annotationId }) => bundleFor(annotationId),
    image: ({ captureId, annotationId, offset }) =>
      image(captureId, offset, annotationId),
    annotate: async ({ captureId, imageDigest, rect, comment }) => {
      const c = getCapture(captureId);
      if (c.image.digest !== imageDigest)
        throw new Error(
          "Версия изображения изменилась. Откройте снимок заново.",
        );
      assertRect(rect, c.image.width, c.image.height);
      const crop = await host.call(
        "cropImage",
        { digest: imageDigest, rect },
        { hostId: c.hostId },
      );
      const a: Annotation = {
        id: randomUUID(),
        captureId,
        imageDigest,
        rect,
        comment,
        createdAt: new Date().toISOString(),
        resolved: false,
        revision: 0,
        crop,
      };
      db.transaction(() => {
        const count = db
          .prepare(
            "SELECT count(*) AS n FROM annotations WHERE capture_id=? AND json_extract(payload, '$.deletedAt') IS NULL",
          )
          .get(captureId) as { n: number };
        if (count.n >= 200)
          throw new Error("На одном снимке можно сохранить до 200 замечаний.");
        db.prepare("INSERT INTO annotations VALUES (?, ?, ?, ?)").run(
          a.id,
          captureId,
          a.createdAt,
          JSON.stringify(a),
        );
      })();
      bb.realtime.publish("review-changed", {
        threadId: c.threadId,
        captureId,
      });
      return a;
    },
    transformAnnotation: async ({
      annotationId,
      revision,
      imageDigest,
      rect,
    }) => {
      const original = getAnnotation(annotationId),
        c = getCapture(original.captureId);
      if (original.revision !== revision)
        throw new Error(
          "Область изменена в другой вкладке. Выберите её заново.",
        );
      if (
        c.image.digest !== imageDigest ||
        original.imageDigest !== imageDigest
      )
        throw new Error("Версия изображения изменилась.");
      assertRect(rect, c.image.width, c.image.height);
      if (
        rect.x === original.rect.x &&
        rect.y === original.rect.y &&
        rect.width === original.rect.width &&
        rect.height === original.rect.height
      )
        return original;
      const crop = await host.call(
        "cropImage",
        { digest: imageDigest, rect },
        { hostId: c.hostId },
      );
      return changeAnnotation(annotationId, revision, (a) => ({
        ...a,
        rect,
        crop,
        resolved: false,
      }));
    },
    updateAnnotation: ({ annotationId, revision, comment, resolved }) =>
      changeAnnotation(annotationId, revision, (a) => ({
        ...a,
        comment,
        ...(resolved === undefined ? {} : { resolved }),
      })),
    deleteAnnotation: ({ annotationId, revision }) =>
      changeAnnotation(annotationId, revision, (a) => ({
        ...a,
        deletedAt: new Date().toISOString(),
      })),
    restoreAnnotation: ({ annotationId, revision }) =>
      changeAnnotation(
        annotationId,
        revision,
        (a) => {
          if (!a.deletedAt) throw new Error("Область уже восстановлена.");
          const count = db
            .prepare(
              "SELECT count(*) AS n FROM annotations WHERE capture_id=? AND json_extract(payload, '$.deletedAt') IS NULL",
            )
            .get(a.captureId) as { n: number };
          if (count.n >= 200) throw new Error("На снимке уже 200 областей.");
          return { ...a, deletedAt: null };
        },
        true,
      ),
    resolve: ({ annotationId, revision, resolved }) => {
      const a = db.transaction(() => {
        const old = getAnnotation(annotationId);
        if (old.revision !== revision)
          throw new Error(
            "Замечание изменилось в другой вкладке. Обновите список.",
          );
        const next = { ...old, resolved, revision: revision + 1 };
        db.prepare("UPDATE annotations SET payload=? WHERE id=?").run(
          JSON.stringify(next),
          annotationId,
        );
        return next;
      })();
      bb.realtime.publish("review-changed", { captureId: a.captureId });
      return a;
    },
  });
  bb.ui.registerMentionProvider({
    id: "region",
    label: "Замечания к изображению",
    search: ({ query, projectId }) => {
      if (!projectId) return [];
      const rows = db
        .prepare(
          "SELECT a.payload FROM annotations a JOIN captures c ON c.id=a.capture_id WHERE c.project_id=? AND json_extract(a.payload, '$.deletedAt') IS NULL ORDER BY a.created_at DESC LIMIT 200",
        )
        .all(projectId) as { payload: string }[];
      return rows
        .map((r) => annotationSchema.parse(JSON.parse(r.payload)))
        .filter(
          (a) =>
            a.comment.toLowerCase().includes(query.toLowerCase()) ||
            a.id.includes(query),
        )
        .slice(0, 12)
        .map((a) => ({
          id: a.id,
          title: a.comment.slice(0, 80),
          subtitle: `${a.rect.width} × ${a.rect.height} px · ${a.id.slice(0, 8)}`,
        }));
    },
    resolve: (annotationId) => ({
      context: `Visual review annotation:\n${describe(bundleFor(annotationId))}\nUse grf_review_read with annotationId "${annotationId}" to SEE the original and crop before diagnosing. If the tool is not available in this session, run bb grf-review export ${annotationId} . and view both exported images. Open in BB with ::grf-review{annotation="${annotationId}"}.`,
    }),
  });
  bb.agents.registerTool({
    name: "grf_review_list",
    description:
      "List game screenshots and visual review annotations for this thread.",
    parameters: z.object({}).strict(),
    execute: (_, ctx) => JSON.stringify(listSummary(ctx.threadId)),
  });
  bb.agents.registerTool({
    name: "grf_review_read",
    description:
      "Read a visual issue: returns the original game screenshot and exact region crop with the user's comment and pixel coordinates. Use before diagnosing an annotated rendering issue.",
    parameters: z.object({ annotationId: id }).strict(),
    presentation: {
      label: {
        pending: "Открываю отмеченную область",
        completed: "Открыта отмеченная область",
      },
    },
    async execute({ annotationId }, ctx): Promise<PluginAgentToolResult> {
      const bundle = bundleFor(annotationId);
      if (bundle.capture.projectId !== ctx.projectId)
        throw new Error("Замечание относится к другому проекту.");
      const original = await host.call(
        "agentImage",
        { digest: bundle.capture.image.digest },
        { hostId: bundle.capture.hostId, signal: ctx.signal },
      );
      const crop = await host.call(
        "agentImage",
        { digest: bundle.annotation.crop.digest },
        { hostId: bundle.capture.hostId, signal: ctx.signal },
      );
      return {
        content: [
          { type: "text", text: describe(bundle) },
          {
            type: "text",
            text: `Full image context, delivered at ${original.width}×${original.height}; source coordinates refer to ${bundle.capture.image.width}×${bundle.capture.image.height}.`,
          },
          { type: "image", ...original.image },
          {
            type: "text",
            text: `Region delivered at ${crop.width}×${crop.height}; original region is ${bundle.annotation.rect.width}×${bundle.annotation.rect.height}. Use CLI export for exact pixels when dimensions differ.`,
          },
          { type: "image", ...crop.image },
        ],
      };
    },
  });
  bb.agents.registerTool({
    name: "grf_search",
    description:
      "Search actual GRF archive contents configured in midgard-ro config.yaml; returns stable resource IDs and archive override chains.",
    parameters: z
      .object({
        query: z.string().max(200).default(""),
        type: z.string().max(12).default(""),
        archive: z.number().int().min(-1).max(15).default(-1),
        offset: z.number().int().nonnegative().default(0),
      })
      .strict(),
    execute: async (input, ctx) =>
      JSON.stringify(
        await searchAssets({ ...input, threadId: ctx.threadId }, ctx.signal),
      ),
  });
  bb.agents.registerTool({
    name: "grf_inspect",
    description:
      "Inspect a GRF SPR/ACT resource: paired file, actions, frame counts and game-client animation intervals.",
    parameters: z
      .object({
        id: digest,
        archive: z.number().int().min(-1).max(15).default(-1),
      })
      .strict(),
    execute: async (input, ctx) =>
      JSON.stringify(
        await inspectAsset({ ...input, threadId: ctx.threadId }, ctx.signal),
      ),
  });
  bb.agents.registerTool({
    name: "grf_render",
    description:
      "Render a GRF bitmap, SPR frame or ACT action using current midgard-ro decoders/compositor. animated=true returns an atlas of up to 256 frames; metadata includes origins, dependencies and renderer source hash.",
    parameters: grfRenderInput,
    execute: async (input, ctx) => {
      const p = await renderAsset(
          { ...input, threadId: ctx.threadId },
          ctx.signal,
        ),
        saved = storedPreview(p.previewId);
      const img = await host.call(
        "agentImage",
        { digest: p.image.digest },
        { hostId: saved.hostId, signal: ctx.signal },
      );
      return {
        content: [
          {
            type: "text" as const,
            text: JSON.stringify({
              ...p,
              deliveredImage: { width: img.width, height: img.height },
            }),
          },
          { type: "image" as const, ...img.image },
        ],
      };
    },
  });
  const usage =
    "bb grf-review assets search <query> [type] [offset] [--json]\nbb grf-review assets inspect <resource-id> [--json]\nbb grf-review assets render <resource-id> [action] [frame] [--json]\nbb grf-review list [--json]\nbb grf-review open <workspace-image-path> [--title <text>] [--context <scene notes>] [--json]\nbb grf-review show <capture-or-annotation-id> [--json]\nbb grf-review export <annotation-id> <existing-workspace-directory> [--json]";
  bb.cli.register({
    name: "grf-review",
    summary:
      "Search and render GRF resources; inspect and export annotated game frames.",
    commands: [
      {
        name: "assets",
        summary: "Search, inspect and render GRF resources",
        usage:
          "bb grf-review assets search <query> [type] [offset] | inspect <id> | render <id> [action] [frame]",
      },
      {
        name: "list",
        summary: "List this thread's captures",
        usage: "bb grf-review list --json",
      },
      {
        name: "open",
        summary: "Import an immutable screenshot",
        usage:
          "bb grf-review open <path> --title <title> --context <notes> --json",
      },
      {
        name: "show",
        summary: "Read a capture or annotation",
        usage: "bb grf-review show <id> --json",
      },
      {
        name: "export",
        summary: "Export original image, crop, and review JSON",
        usage:
          "bb grf-review export <annotation-id> <existing-workspace-directory> --json",
      },
    ],
    async run(argv, ctx) {
      try {
        if (argv[0] === "assets") {
          if (!ctx.threadId) throw new Error("Run inside a BB project thread");
          const [, op, arg, arg2, arg3] = argv.filter((a) => a !== "--json");
          let result: unknown;
          if (op === "search")
            result = await searchAssets(
              {
                threadId: ctx.threadId,
                query: arg ?? "",
                type: arg2 ?? "",
                archive: -1,
                offset: Number(arg3 ?? 0),
              },
              ctx.signal,
            );
          else if (op === "inspect" && arg)
            result = await inspectAsset(
              { threadId: ctx.threadId, id: arg, archive: -1 },
              ctx.signal,
            );
          else if (op === "render" && arg)
            result = await renderAsset(
              {
                threadId: ctx.threadId,
                id: arg,
                archive: -1,
                action: Number(arg2 ?? 0),
                frame: Number(arg3 ?? 0),
                animated: false,
                colorKey: true,
              },
              ctx.signal,
            );
          else
            throw new Error(
              "bb grf-review assets search <query> [type] [offset] | inspect <id> | render <id> [action] [frame]",
            );
          return { exitCode: 0, stdout: JSON.stringify(result, null, 2) };
        }
        const args: string[] = [],
          opts: Record<string, string> = {};
        for (let i = 0; i < argv.length; i++) {
          const arg = argv[i]!;
          if (arg === "--json") continue;
          if (arg === "--title" || arg === "--context") {
            if (!argv[i + 1]) throw new Error(`Missing ${arg} value`);
            opts[arg] = argv[++i]!;
          } else if (arg.startsWith("--") && arg !== "--help")
            throw new Error(`Unknown option ${arg}`);
          else args.push(arg);
        }
        const [command, target, directory] = args;
        if (!command || command === "help" || command === "--help")
          return { exitCode: 0, stdout: usage };
        if (!ctx.threadId)
          throw new Error(
            "Запустите команду из обсуждения bb с рабочим окружением.",
          );
        let result: unknown;
        if (command === "list" && args.length === 1)
          result = listSummary(ctx.threadId);
        else if (command === "open" && target && args.length === 2)
          result = await importImage(
            {
              threadId: ctx.threadId,
              path: target,
              title: opts["--title"] ?? target.split(/[\\/]/).pop()!,
              sceneContext: opts["--context"] ?? "",
            },
            ctx.signal,
          );
        else if (command === "show" && target && args.length === 2) {
          id.parse(target);
          const exists = db
            .prepare("SELECT id FROM captures WHERE id=?")
            .get(target);
          const capture = exists
            ? getCapture(target)
            : bundleFor(target).capture;
          const loc = await location(ctx.threadId);
          if (capture.projectId !== loc.projectId)
            throw new Error("Другой проект.");
          result = exists
            ? {
                capture,
                annotations: annotationsFor(target).slice(-20),
                annotationCount: annotationsFor(target).length,
              }
            : bundleFor(target);
        } else if (
          command === "export" &&
          target &&
          directory &&
          args.length === 3
        ) {
          const bundle = bundleFor(target),
            loc = await location(ctx.threadId);
          if (
            bundle.capture.projectId !== loc.projectId ||
            bundle.capture.hostId !== loc.hostId
          )
            throw new Error(
              "Экспортируйте из окружения этого проекта на машине исходного снимка.",
            );
          result = await host.call(
            "exportAnnotation",
            { root: loc.root, directory, bundle },
            { hostId: loc.hostId, signal: ctx.signal },
          );
        } else throw new Error(usage);
        return { exitCode: 0, stdout: JSON.stringify(result, null, 2) };
      } catch (error) {
        return {
          exitCode: 1,
          stderr: error instanceof Error ? error.message : String(error),
        };
      }
    },
  });
}
