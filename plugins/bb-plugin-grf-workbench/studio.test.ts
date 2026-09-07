import { expect, it } from "vitest";
import {
  createFakePluginHost,
  makeThreadResponse,
} from "@get-bb/plugin-sdk/testing";
import plugin, { describe } from "./server";
import { defaultStudioScene, studioScene } from "./studio-contract";
import type { Capture, Annotation } from "./contract";

import { frameFixture } from "./test-support/studio";

it("validates bounded native scene inputs including unknown fields and non-finite camera values", () => {
  for (const scene of [
    { ...defaultStudioScene, tick: 601 },
    { ...defaultStudioScene, hits: -1 },
    {
      ...defaultStudioScene,
      camera: { ...defaultStudioScene.camera, yaw: Infinity },
    },
    { ...defaultStudioScene, execute: "code" },
  ]) {
    expect(studioScene.safeParse(scene).success).toBe(false);
  }
  expect(studioScene.safeParse(defaultStudioScene).success).toBe(true);
});
it("captures the displayed native frame, persists its context across reload and includes it in agent annotations", async () => {
  const frame = frameFixture();
  const im = {
    digest: "e".repeat(64),
    width: 960,
    height: 640,
    mimeType: "image/png" as const,
    bytes: 100,
  };
  const calls: { method: string; input: unknown; hostId?: string }[] = [];
  const f = createFakePluginHost({
    pluginId: "grf-workbench",
    experimental_hostEntry: true,
    sdk: {
      threads: {
        get: async () =>
          makeThreadResponse({
            id: "thread-1",
            environmentId: "env-1",
            projectId: "project-1",
          }),
      },
      environments: {
        get: async () =>
          ({
            id: "env-1",
            hostId: "remote-host",
            path: "/project",
            projectId: "project-1",
          }) as never,
      },
    },
    experimental_callHostRpc: async (call) => {
      calls.push(call);
      switch (call.method) {
        case "studioRender":
          return frame;
        case "studioCapture":
          expect(call.input).toEqual({
            threadId: "thread-1",
            sessionId: "11111111-1111-4111-8111-111111111111",
            frameId: frame.frameId,
          });
          return { image: im, context: frame.context };
        case "cropImage":
          return { ...im, digest: "f".repeat(64), width: 10, height: 10 };
        default:
          throw new Error(call.method);
      }
    },
  });
  try {
    await plugin(f.bb);
    const scope = {
      threadId: "thread-1",
      sessionId: "11111111-1111-4111-8111-111111111111",
    };
    await f.harness.behavior.callRpc("studioRender", {
      ...scope,
      scene: defaultStudioScene,
    });
    const capture = (await f.harness.behavior.callRpc("studioCapture", {
      ...scope,
      frameId: frame.frameId,
    })) as Capture;
    expect(capture.skillFrame?.tick).toBe(26); // native response, not requested tick 0
    expect(calls.every((c) => c.hostId === "remote-host")).toBe(true);
    expect(calls[0].input).toMatchObject({
      root: "/project",
      scene: defaultStudioScene,
    });
    const annotation = (await f.harness.behavior.callRpc("annotate", {
      captureId: capture.id,
      imageDigest: im.digest,
      rect: { x: 1, y: 2, width: 10, height: 10 },
      comment: "Слишком большой хвост",
    })) as Annotation;
    f.harness = (await f.harness.lifecycle.reload(plugin)).harness;
    const restored = (await f.harness.behavior.callRpc("get", {
      captureId: capture.id,
    })) as { capture: Capture };
    expect(restored.capture.skillFrame).toEqual(frame.context);
    expect(
      JSON.parse(describe({ capture: restored.capture, annotation })).skillFrame
        .rendererVersion,
    ).toBe("c".repeat(64));
  } finally {
    await f.harness.lifecycle.dispose();
  }
});
