import { afterEach, expect, it } from "vitest";
import {
  createFakePluginHost,
  makeThreadResponse,
} from "@get-bb/plugin-sdk/testing";
import type { BbPluginApi, NewThreadRequest } from "@get-bb/plugin-sdk";
import plugin from "./server";
import type { Capture, Annotation, ReviewDraft } from "./contract";

const cleanup: (() => Promise<unknown>)[] = [];
afterEach(async () => {
  for (const f of cleanup.splice(0).reverse()) await f();
});
const req: NewThreadRequest = {
  projectId: "new-project",
  providerId: "codex",
  model: "chosen-model",
  reasoningLevel: "high",
  permissionMode: "accept-edits",
  serviceTier: "fast",
  executionInputSources: {
    providerId: "explicit",
    model: "explicit",
    reasoningLevel: "explicit",
    permissionMode: "explicit",
    serviceTier: "explicit",
  },
  environment: { type: "reuse", environmentId: "chosen-env" },
  input: [{ type: "text", text: "Исправь только отмеченное.", mentions: [] }],
};
async function setup(
  options: { loseSpawn?: boolean; failPublish?: boolean } = {},
) {
  let draft: ReviewDraft | undefined,
    spawnCount = 0,
    updateCount = 0;
  let created: ReturnType<typeof makeThreadResponse> | undefined;
  const image = {
    digest: "a".repeat(64),
    width: 32,
    height: 24,
    mimeType: "image/png" as const,
    bytes: 100,
  };
  const f = createFakePluginHost({
    pluginId: "grf-workbench",
    experimental_hostEntry: true,
    sdk: {
      threads: {
        get: async () =>
          makeThreadResponse({
            id: "source-thread",
            projectId: "source-project",
            environmentId: "source-env",
          }),
        spawn: async (args) => {
          spawnCount++;
          expect(args.environment).toEqual(req.environment);
          expect(args.model).toBe(req.model);
          expect(args.executionInputSources).toEqual(req.executionInputSources);
          expect(args.input?.[0]).toEqual(req.input[0]);
          expect(args.input?.some((x) => x.type === "image")).toBe(true);
          const copy = (await f.harness.behavior.callRpc("get", {
            captureId: draft!.capture.id,
          })) as { capture: Capture; annotations: Annotation[] };
          expect(copy.annotations).toHaveLength(2);
          expect(copy.capture.pendingReview).toBe(draft!.id);
          const originalList = (await f.harness.behavior.callRpc("list", {
            threadId: "source-thread",
          })) as Capture[];
          expect(originalList.some((c) => c.id === copy.capture.id)).toBe(
            false,
          );
          created = makeThreadResponse({
            id: "new-thread",
            title: args.title!,
            projectId: req.projectId,
            environmentId: "chosen-env",
          });
          if (options.loseSpawn)
            throw new Error("Lost response after BB created the thread");
          return created;
        },
        update: async (args) => {
          updateCount++;
          if (options.failPublish && updateCount === 1)
            throw new Error("Publish interrupted");
          created = { ...created!, title: args.title ?? created!.title };
          return created;
        },
        list: async () =>
          created
            ? ([created] as unknown as Awaited<
                ReturnType<BbPluginApi["sdk"]["threads"]["list"]>
              >)
            : [],
      },
      environments: {
        get: async () =>
          ({
            id: "source-env",
            path: "/workspace",
            hostId: "source-host",
            projectId: "source-project",
          }) as never,
      },
    },
    experimental_callHostRpc: async (call) => {
      switch (call.method) {
        case "importImage":
          return { sourcePath: "/workspace/scene.png", image };
        case "cropImage":
          return { ...image, width: 3, height: 3, digest: "b".repeat(64) };
        case "agentImage":
          return {
            image: { mimeType: "image/png", data: "aGVsbG8=" },
            width: 32,
            height: 24,
          };
        default:
          throw new Error(call.method);
      }
    },
  });
  cleanup.push(() => f.harness.lifecycle.dispose());
  await plugin(f.bb);
  const capture = (await f.harness.behavior.callRpc("importImage", {
    threadId: "source-thread",
    path: "scene.png",
    title: "Soul Strike",
    sceneContext: "Rocker",
  })) as Capture;
  const annotations: Annotation[] = [];
  for (const comment of ["Первое замечание", "Второе замечание"])
    annotations.push(
      (await f.harness.behavior.callRpc("annotate", {
        captureId: capture.id,
        imageDigest: image.digest,
        rect: { x: 1, y: 2, width: 3, height: 3 },
        comment,
      })) as Annotation,
    );
  const input = {
    operationId: "11111111-1111-4111-8111-111111111111",
    captureId: capture.id,
    imageDigest: image.digest,
    title: "Независимый разбор",
    annotations: annotations.map((a) => ({ id: a.id, revision: a.revision })),
  };
  draft = (await f.harness.behavior.callRpc(
    "prepareReview",
    input,
  )) as ReviewDraft;
  return {
    f,
    capture,
    annotations,
    draft,
    input,
    counts: () => ({ spawnCount, updateCount }),
  };
}
it("copies selected comments before starting, preserves selections, and survives deletion of the original", async () => {
  const { f, capture, annotations, draft, input, counts } = await setup();
  expect(await f.harness.behavior.callRpc("prepareReview", input)).toEqual(
    draft,
  );
  expect(draft.defaultEnvironment).toEqual({
    type: "reuse",
    environmentId: "source-env",
  });
  expect(draft.capture.id).not.toBe(capture.id);
  expect(draft.annotations.map((a) => a.id)).not.toEqual(
    annotations.map((a) => a.id),
  );
  await f.harness.behavior.callRpc("deleteCapture", {
    captureId: capture.id,
    imageDigest: capture.image.digest,
  });
  const [first, second] = await Promise.all([
    f.harness.behavior.callRpc("startReview", {
      operationId: draft.id,
      request: req,
    }),
    f.harness.behavior.callRpc("startReview", {
      operationId: draft.id,
      request: req,
    }),
  ]);
  expect(first).toEqual(second);
  expect(counts().spawnCount).toBe(1);
  const copy = (await f.harness.behavior.callRpc("get", {
    captureId: draft.capture.id,
  })) as { capture: Capture; annotations: Annotation[] };
  expect(copy.capture.threadId).toBe("new-thread");
  expect(copy.capture.projectId).toBe(req.projectId);
  expect(copy.capture.pendingReview).toBeUndefined();
  expect(copy.annotations.map((a) => a.comment)).toEqual(
    annotations.map((a) => a.comment),
  );
  expect(copy.capture.copiedFrom?.captureId).toBe(capture.id);
  expect(
    await f.harness.behavior.callRpc("reviewForThread", {
      threadId: "new-thread",
    }),
  ).toEqual({ captureId: draft.capture.id, title: draft.title });
  const reload = await f.harness.lifecycle.reload(plugin);
  f.harness = reload.harness;
  cleanup.push(() => reload.harness.lifecycle.dispose());
  expect(
    await f.harness.behavior.callRpc("startReview", { operationId: draft.id }),
  ).toEqual(first);
  expect(counts().spawnCount).toBe(1);
});
it("recovers an acknowledged-by-BB create after losing its response without spawning again", async () => {
  const { f, draft, counts } = await setup({ loseSpawn: true });
  await expect(
    f.harness.behavior.callRpc("startReview", {
      operationId: draft.id,
      request: req,
    }),
  ).rejects.toThrow("Lost response");
  const reload = await f.harness.lifecycle.reload(plugin);
  f.harness = reload.harness;
  cleanup.push(() => reload.harness.lifecycle.dispose());
  expect(
    await f.harness.behavior.callRpc("startReview", { operationId: draft.id }),
  ).toEqual({ threadId: "new-thread", captureId: draft.capture.id });
  expect(counts().spawnCount).toBe(1);
});
it("retries publishing the same prepared thread and rejects changed annotation revisions", async () => {
  const { f, draft, input, annotations, counts } = await setup({
    failPublish: true,
  });
  await expect(
    f.harness.behavior.callRpc("prepareReview", {
      ...input,
      operationId: "22222222-2222-4222-8222-222222222222",
      annotations: [{ id: annotations[0].id, revision: 99 }],
    }),
  ).rejects.toThrow("Замечания изменились");
  await expect(
    f.harness.behavior.callRpc("startReview", {
      operationId: draft.id,
      request: req,
    }),
  ).rejects.toThrow("Publish interrupted");
  expect(
    await f.harness.behavior.callRpc("startReview", { operationId: draft.id }),
  ).toEqual({ threadId: "new-thread", captureId: draft.capture.id });
  expect(counts()).toEqual({ spawnCount: 1, updateCount: 2 });
});

it("discards an unsent draft without touching its source, but keeps a started operation recoverable", async () => {
  const { f, draft, capture, counts } = await setup();
  expect(
    await f.harness.behavior.callRpc("discardReviewDraft", {
      operationId: draft.id,
    }),
  ).toEqual({ discarded: true });
  await expect(
    f.harness.behavior.callRpc("reviewDraft", { operationId: draft.id }),
  ).rejects.toThrow("Черновик разбора не найден");
  expect(
    await f.harness.behavior.callRpc("get", { captureId: capture.id }),
  ).toMatchObject({ capture: { id: capture.id } });
  expect(counts().spawnCount).toBe(0);
  const started = await setup({ loseSpawn: true });
  await expect(
    started.f.harness.behavior.callRpc("startReview", {
      operationId: started.draft.id,
      request: req,
    }),
  ).rejects.toThrow("Lost response");
  await expect(
    started.f.harness.behavior.callRpc("discardReviewDraft", {
      operationId: started.draft.id,
    }),
  ).rejects.toThrow("уже началось");
});
