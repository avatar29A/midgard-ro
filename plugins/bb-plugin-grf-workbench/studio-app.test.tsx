// @vitest-environment jsdom
import { expect, it, vi } from "vitest";
import { fireEvent, waitFor } from "@testing-library/react";
import { loadPluginApp, renderSlot } from "@get-bb/plugin-sdk/testing/app";
import { frameFixture } from "./test-support/studio";
import type { StudioScene } from "./studio-contract";
it("rotates a paused native scene, captures its displayed token and closes the worker on leaving Studio", async () => {
  vi.stubGlobal(
    "ResizeObserver",
    class {
      observe() {}
      unobserve() {}
      disconnect() {}
    },
  );
  HTMLImageElement.prototype.decode = async () => {};
  const frame = frameFixture();
  const render = vi.fn(async (input: { scene: StudioScene }) => ({
    ...frame,
    context: { ...frame.context, scene: input.scene, tick: input.scene.tick },
  }));
  const close = vi.fn(() => ({ closed: true }));
  const capture = {
    id: "22222222-2222-4222-8222-222222222222",
    threadId: "thread-1",
    projectId: "p1",
    hostId: "h1",
    title: "Soul Strike capture",
    sceneContext: "volley only",
    sourcePath: "skillstudio://test",
    createdAt: "2026-09-07",
    image: {
      digest: "e".repeat(64),
      width: 960,
      height: 640,
      mimeType: "image/png",
      bytes: 10,
    },
    skillFrame: frame.context,
  };
  const captureCall = vi.fn(() => capture);
  const app = await loadPluginApp(() => import("./app"));
  const slot = renderSlot(
    app.threadPanelActions[0]!,
    { threadId: "thread-1", params: { section: "studio" } },
    {
      context: { threadId: "thread-1", projectId: "p1" },
      rpc: {
        studioRender: (i) => render(i as { scene: StudioScene }),
        studioClose: close,
        studioCapture: captureCall,
        list: () => [capture],
        get: () => ({ capture, annotations: [] }),
        image: () => ({
          mimeType: "image/png",
          data: "aGVsbG8=",
          nextOffset: 0,
          done: true,
        }),
      },
    },
  );
  try {
    await slot.findByAltText("Soul Strike · tick 0 · 0°");
    fireEvent.click(slot.getByText("45° ↷"));
    await slot.findByAltText("Soul Strike · tick 0 · 45°");
    expect(render.mock.calls.at(-1)?.[0].scene.tick).toBe(0);
    fireEvent.click(slot.getByLabelText("Следующий tick"));
    await slot.findByAltText("Soul Strike · tick 1 · 45°");
    fireEvent.click(slot.getByText("Снимок и замечание"));
    await waitFor(() => expect(captureCall).toHaveBeenCalled());
    expect(
      slot.inspection.rpcCalls.find((c) => c.method === "studioCapture")?.input,
    ).toMatchObject({ frameId: frame.frameId, threadId: "thread-1" });
    await waitFor(() => expect(close).toHaveBeenCalled());
  } finally {
    slot.lifecycle.unmount();
    vi.unstubAllGlobals();
  }
});

it("keeps showing completed frames during continuous camera input and coalesces pending requests", async () => {
  HTMLImageElement.prototype.decode = async () => {};
  const frame = frameFixture();
  const pending: { scene: StudioScene; resolve: (result: unknown) => void }[] =
    [];
  let active = 0,
    maxActive = 0;
  const app = await loadPluginApp(() => import("./app"));
  const slot = renderSlot(
    app.threadPanelActions[0]!,
    { threadId: "thread-1", params: { section: "studio" } },
    {
      context: { threadId: "thread-1", projectId: "p1" },
      rpc: {
        studioRender: (input) =>
          new Promise((resolve) => {
            active++;
            maxActive = Math.max(maxActive, active);
            pending.push({
              scene: (input as { scene: StudioScene }).scene,
              resolve: (value) => {
                active--;
                resolve(value);
              },
            });
          }),
        studioClose: () => ({ closed: true }),
      },
    },
  );
  async function finish(yaw: number) {
    await waitFor(() => expect(pending.length).toBe(1));
    const request = pending.shift()!;
    expect(request.scene.camera.yaw).toBe(yaw);
    const png = btoa(JSON.stringify(request.scene));
    request.resolve({
      ...frame,
      png,
      context: { ...frame.context, tick: 0, scene: request.scene },
    });
    await slot.findByAltText(`Soul Strike · tick 0 · ${yaw}°`);
    await waitFor(() =>
      expect(slot.getByRole("img").getAttribute("src")).toBe(
        `data:image/png;base64,${png}`,
      ),
    );
  }
  try {
    await finish(0);
    fireEvent.click(slot.getByText("45° ↷"));
    await waitFor(() => expect(pending.length).toBe(1));
    // New input arrives while the previous native frame is still rendering.
    // The completed 45° frame must remain useful, rather than be discarded.
    fireEvent.click(slot.getByText("45° ↷"));
    fireEvent.click(slot.getByText("45° ↷"));
    await finish(45);
    await waitFor(() => expect(pending.length).toBe(1));
    expect(pending[0].scene.camera.yaw).toBe(135); // no queue of 90° frames
    fireEvent.click(slot.getByText("45° ↷"));
    await finish(135);
    await finish(180);
    const view = slot.getByLabelText("Сцена Skill Studio");
    fireEvent.wheel(view, { deltaY: 30 });
    await waitFor(() => expect(pending.length).toBe(1));
    const firstZoom = pending[0].scene.camera.distance;
    fireEvent.wheel(view, { deltaY: 30 });
    await finish(180);
    await waitFor(() => expect(pending.length).toBe(1));
    expect(pending[0].scene.camera.distance).toBeGreaterThan(firstZoom);
    await finish(180);
    expect(maxActive).toBe(1);
  } finally {
    slot.lifecycle.unmount();
  }
});

it("starts playback at the paused tick without counting time spent idle", async () => {
  HTMLImageElement.prototype.decode = async () => {};
  let now = 0;
  const clock = vi.spyOn(performance, "now").mockImplementation(() => now);
  const frame = frameFixture();
  const app = await loadPluginApp(() => import("./app"));
  const render = vi.fn(async (input: { scene: StudioScene }) => ({
    ...frame,
    context: { ...frame.context, scene: input.scene, tick: input.scene.tick },
  }));
  const slot = renderSlot(
    app.threadPanelActions[0]!,
    { threadId: "thread-1", params: { section: "studio" } },
    {
      context: { threadId: "thread-1", projectId: "p1" },
      rpc: {
        studioRender: (input) => render(input as { scene: StudioScene }),
        studioClose: () => ({ closed: true }),
      },
    },
  );
  try {
    await slot.findByAltText("Soul Strike · tick 0 · 0°");
    now = 10000; // ten seconds looking at the paused frame
    fireEvent.click(slot.getByText("Play"));
    now += 17;
    await slot.findByAltText("Soul Strike · tick 1 · 0°");
    expect(render.mock.calls[1][0].scene.tick).toBe(1);
  } finally {
    slot.lifecycle.unmount();
    clock.mockRestore();
  }
});
