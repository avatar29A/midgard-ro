// @vitest-environment jsdom
import { expect, it, vi } from "vitest";
import { fireEvent, waitFor } from "@testing-library/react";
import { loadPluginApp, renderSlot } from "@get-bb/plugin-sdk/testing/app";
import type { Capture, Annotation, ReviewDraft } from "./contract";
const original: Capture = {
  id: "11111111-1111-4111-8111-111111111111",
  threadId: "source-thread",
  projectId: "source-project",
  hostId: "host-1",
  title: "Soul Strike · 56",
  sceneContext: "Rocker",
  sourcePath: "scene.png",
  createdAt: "2026-09-08",
  image: {
    digest: "a".repeat(64),
    width: 100,
    height: 100,
    mimeType: "image/png",
    bytes: 100,
  },
};
const annotations: Annotation[] = [false, true].map((resolved, i) => ({
  id: `${i + 2}`.repeat(8) + "-1111-4111-8111-111111111111",
  captureId: original.id,
  imageDigest: original.image.digest,
  rect: { x: 5, y: 5, width: 10, height: 10 },
  comment: i ? "Уже проверено" : "Слишком длинный хвост",
  createdAt: "2026-09-08",
  resolved,
  revision: i,
  crop: { ...original.image, width: 10, height: 10 },
}));
const draft: ReviewDraft = {
  id: "44444444-1111-4111-8111-111111111111",
  title: "Разбор Soul Strike",
  capture: {
    ...original,
    id: "55555555-1111-4111-8111-111111111111",
    copiedFrom: {
      captureId: original.id,
      threadId: original.threadId,
      copiedAt: "2026-09-08",
    },
  },
  annotations: [annotations[0]],
  defaultEnvironment: { type: "reuse", environmentId: "env-source" },
  status: "prepared",
  threadId: null,
};
it("selects unresolved comments and opens the host composer with a stable draft", async () => {
  const app = await loadPluginApp(() => import("./app"));
  const prepare = vi.fn(() => draft);
  const slot = renderSlot(
    app.navPanels[0]!,
    { subPath: `new/${original.id}` },
    {
      rpc: {
        get: () => ({ capture: original, annotations }),
        image: () => ({
          mimeType: "image/png",
          data: "aGVsbG8=",
          nextOffset: 0,
          done: true,
        }),
        prepareReview: prepare,
      },
    },
  );
  try {
    const first = (await slot.findByLabelText(
      "Включить замечание 1",
    )) as HTMLInputElement;
    expect(first.checked).toBe(true);
    expect(
      (slot.getByLabelText("Включить замечание 2") as HTMLInputElement).checked,
    ).toBe(false);
    fireEvent.click(slot.getByText("Подготовить запрос · 1 замечаний"));
    await waitFor(() => expect(prepare).toHaveBeenCalledTimes(1));
    expect(
      slot.inspection.rpcCalls.find((c) => c.method === "prepareReview")?.input,
    ).toMatchObject({
      captureId: original.id,
      annotations: [{ id: annotations[0].id, revision: 0 }],
    });
    expect(slot.inspection.navigateCalls).toContainEqual(
      expect.objectContaining({
        method: "toPluginPanel",
        path: "review",
        options: { subPath: `draft/${draft.id}`, replace: true },
      }),
    );
  } finally {
    slot.lifecycle.unmount();
  }
});
it("uses BB's composer, forwards its request, then opens the independent capture on the new thread surface", async () => {
  const app = await loadPluginApp(() => import("./app"));
  const start = vi.fn(() => ({
    threadId: "new-thread",
    captureId: draft.capture.id,
  }));
  const slot = renderSlot(
    app.navPanels[0]!,
    { subPath: `draft/${draft.id}` },
    { rpc: { reviewDraft: () => draft, startReview: start } },
  );
  try {
    const composer = await slot.findByTestId("bb-new-thread-composer");
    expect(composer.getAttribute("data-default-project-id")).toBe(
      original.projectId,
    );
    expect(
      JSON.parse(composer.getAttribute("data-default-environment")!),
    ).toEqual(draft.defaultEnvironment);
    expect(
      (slot.getByTestId("bb-new-thread-composer-input") as HTMLTextAreaElement)
        .value,
    ).toContain(annotations[0].comment);
    fireEvent.change(slot.getByTestId("bb-new-thread-composer-input"), {
      target: { value: "Мой уточнённый запрос" },
    });
    fireEvent.click(slot.getByTestId("bb-new-thread-composer-submit"));
    await waitFor(() => expect(start).toHaveBeenCalledTimes(1));
    expect(
      slot.inspection.rpcCalls.find((c) => c.method === "startReview")?.input,
    ).toMatchObject({
      operationId: draft.id,
      request: {
        input: [{ type: "text", text: "Мой уточнённый запрос", mentions: [] }],
      },
    });
    expect(slot.inspection.navigateCalls).toContainEqual(
      expect.objectContaining({ method: "toThread", threadId: "new-thread" }),
    );
  } finally {
    slot.lifecycle.unmount();
  }
  const header = renderSlot(
    app.threadHeaderActions[0]!,
    {
      threadId: "new-thread",
      projectId: original.projectId,
      isCompactViewport: false,
    },
    {
      rpc: {
        reviewForThread: () => ({
          captureId: draft.capture.id,
          title: draft.title,
        }),
      },
      context: { threadId: "new-thread", projectId: original.projectId },
    },
  );
  try {
    await header.findByRole("button", { name: "Открыть снимок разбора" });
    await waitFor(() =>
      expect(
        header.inspection.navigateCalls.some(
          (c) => c.method === "openThreadPanel",
        ),
      ).toBe(true),
    );
    expect(
      header.inspection.navigateCalls.find(
        (c) => c.method === "openThreadPanel",
      )?.options,
    ).toEqual({
      actionId: "review",
      title: draft.title,
      params: { captureId: draft.capture.id, section: "review" },
    });
  } finally {
    header.lifecycle.unmount();
    sessionStorage.clear();
  }
});
