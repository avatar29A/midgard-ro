// @vitest-environment jsdom
import { expect, it, vi } from "vitest";
import { fireEvent, waitFor } from "@testing-library/react";
import { loadPluginApp, renderSlot } from "@get-bb/plugin-sdk/testing/app";
import type { Annotation, Capture } from "./contract";
const capture: Capture = {
  id: "11111111-1111-4111-8111-111111111111",
  threadId: "thread-1",
  projectId: "project-1",
  hostId: "host-1",
  title: "Wall screenshot",
  sceneContext: "Prontera",
  sourcePath: "scene.png",
  createdAt: "2026-09-07",
  image: {
    digest: "a".repeat(64),
    width: 200,
    height: 100,
    mimeType: "image/png",
    bytes: 30,
  },
};
it("saves the exact drag rectangle and adds a durable annotation mention without submitting the user's draft", async () => {
  vi.stubGlobal(
    "ResizeObserver",
    class {
      observe() {}
      unobserve() {}
      disconnect() {}
    },
  );
  class Pointer extends MouseEvent {
    pointerId = 1;
  }
  vi.stubGlobal("PointerEvent", Pointer);
  HTMLElement.prototype.setPointerCapture = () => {};
  HTMLElement.prototype.releasePointerCapture = () => {};
  HTMLElement.prototype.hasPointerCapture = () => true;
  const app = await loadPluginApp(() => import("./app"));
  let annotations: Annotation[] = [];
  const annotate = vi.fn(
    async (input: {
      captureId: string;
      imageDigest: string;
      rect: Annotation["rect"];
      comment: string;
    }) => {
      const a: Annotation = {
        ...input,
        id: "22222222-2222-4222-8222-222222222222",
        createdAt: "2026-09-07",
        resolved: false,
        revision: 0,
        crop: {
          digest: "b".repeat(64),
          width: input.rect.width,
          height: input.rect.height,
          mimeType: "image/png",
          bytes: 25,
        },
      };
      annotations = [a];
      return a;
    },
  );
  const slot = renderSlot(
    app.threadPanelActions[0]!,
    { threadId: "thread-1", params: { captureId: capture.id } },
    {
      context: { threadId: "thread-1", projectId: "project-1" },
      composer: {
        text: "Existing draft",
        scope: { kind: "thread", threadId: "thread-1" },
      },
      rpc: {
        list: () => [capture],
        get: () => ({ capture, annotations }),
        image: () => ({
          mimeType: "image/png",
          data: "aGVsbG8=",
          nextOffset: 5,
          done: true,
        }),
        annotate: (input) => annotate(input as Parameters<typeof annotate>[0]),
      },
    },
  );
  try {
    const img = await slot.findByAltText("Wall screenshot");
    vi.spyOn(img, "getBoundingClientRect").mockReturnValue({
      left: 10,
      top: 10,
      width: 100,
      height: 50,
      right: 110,
      bottom: 60,
      x: 10,
      y: 10,
      toJSON: () => ({}),
    });
    const area = img.parentElement!;
    fireEvent.pointerDown(area, { clientX: 20, clientY: 20, button: 0 });
    fireEvent.pointerMove(area, { clientX: 50, clientY: 40, button: 0 });
    fireEvent.pointerUp(area, { clientX: 50, clientY: 40, button: 0 });
    const view = slot.getByLabelText("Просмотр снимка");
    fireEvent.keyDown(view, { key: "r", code: "KeyR" });
    expect(
      slot.getByLabelText("Изменить размер: правый нижний угол"),
    ).toBeTruthy();
    fireEvent.keyDown(view, { key: "Escape" });
    const comment = await slot.findByLabelText("Комментарий к области");
    fireEvent.change(comment, {
      target: { value: "NPC is clipped by this wall" },
    });
    fireEvent.click(slot.getByText("Сохранить замечание"));
    await waitFor(() =>
      expect(annotate).toHaveBeenCalledWith({
        captureId: capture.id,
        imageDigest: capture.image.digest,
        rect: { x: 20, y: 20, width: 60, height: 40 },
        comment: "NPC is clipped by this wall",
      }),
    );
    fireEvent.click(await slot.findByText("В обсуждение"));
    expect(slot.inspection.composer.mentions[0]).toMatchObject({
      provider: "region",
      id: annotations[0]!.id,
    });
    expect(slot.inspection.composer.text).toContain("Existing draft");
    expect(slot.inspection.composer.submits).toHaveLength(0);
  } finally {
    slot.unmount();
    vi.unstubAllGlobals();
  }
});

it("zooms around the cursor and pans without losing a region or intercepting spaces in comments", async () => {
  vi.stubGlobal(
    "ResizeObserver",
    class {
      observe() {}
      disconnect() {}
    },
  );
  vi.stubGlobal(
    "PointerEvent",
    class extends MouseEvent {
      pointerId = 1;
    },
  );
  HTMLElement.prototype.setPointerCapture = () => {};
  HTMLElement.prototype.releasePointerCapture = () => {};
  HTMLElement.prototype.hasPointerCapture = () => true;
  const app = await loadPluginApp(() => import("./app"));
  const slot = renderSlot(
    app.threadPanelActions[0]!,
    { threadId: "thread-1", params: { captureId: capture.id } },
    {
      rpc: {
        list: () => [capture],
        get: () => ({ capture, annotations: [] }),
        image: () => ({
          mimeType: "image/png",
          data: "aGVsbG8=",
          nextOffset: 5,
          done: true,
        }),
      },
    },
  );
  try {
    const img = await slot.findByAltText("Wall screenshot");
    const area = img.parentElement!;
    const view = slot.getByLabelText("Просмотр снимка");
    const zoom = slot.getByLabelText("Масштаб") as HTMLSelectElement;
    vi.spyOn(img, "getBoundingClientRect").mockImplementation(() => {
      const width = parseFloat(area.style.width),
        height = parseFloat(area.style.height);
      const left = 16 - view.scrollLeft,
        top = 16 - view.scrollTop;
      return {
        left,
        top,
        width,
        height,
        right: left + width,
        bottom: top + height,
        x: left,
        y: top,
        toJSON: () => ({}),
      };
    });
    fireEvent.change(zoom, { target: { value: "4" } });
    view.scrollLeft = 200;
    view.scrollTop = 60;
    const pointUnderCursor = () => {
      const r = img.getBoundingClientRect();
      return {
        x: ((80 - r.left) * 200) / r.width,
        y: ((50 - r.top) * 100) / r.height,
      };
    };
    const before = pointUnderCursor();
    expect(
      fireEvent.wheel(view, { deltaY: -100, clientX: 80, clientY: 50 }),
    ).toBe(false);
    expect(Number(zoom.value)).toBeGreaterThan(4);
    expect(pointUnderCursor().x).toBeCloseTo(before.x, 9);
    expect(pointUnderCursor().y).toBeCloseTo(before.y, 9);
    fireEvent.wheel(view, { deltaY: 100, clientX: 80, clientY: 50 });
    expect(Number(zoom.value)).toBeCloseTo(4);
    fireEvent.pointerDown(area, { clientX: 60, clientY: 40, button: 0 });
    fireEvent.pointerUp(area, { clientX: 100, clientY: 80, button: 0 });
    const coordinates = slot.getByTestId("selection-coordinates").textContent;
    const comment = slot.getByLabelText("Комментарий к области");
    fireEvent.change(comment, { target: { value: "Keep this comment" } });
    // Middle-button drag keeps select mode and the unsaved region.
    let left = view.scrollLeft,
      top = view.scrollTop;
    fireEvent.pointerDown(area, { clientX: 90, clientY: 80, button: 1 });
    fireEvent.pointerMove(area, { clientX: 60, clientY: 60, button: 1 });
    fireEvent.pointerUp(area, { clientX: 60, clientY: 60, button: 1 });
    expect(view.scrollLeft).toBeCloseTo(left + 30);
    expect(view.scrollTop).toBeCloseTo(top + 20);
    expect(slot.getByTestId("selection-coordinates").textContent).toBe(
      coordinates,
    );
    // Holding Space before clicking uses temporary pan; releasing it mid-drag
    // must not switch this gesture into a rectangle selection.
    fireEvent.pointerEnter(view);
    fireEvent.keyDown(view, { key: " ", code: "Space" });
    left = view.scrollLeft;
    fireEvent.pointerDown(area, { clientX: 90, clientY: 80, button: 0 });
    fireEvent.keyUp(window, { key: " ", code: "Space" });
    fireEvent.pointerUp(area, { clientX: 70, clientY: 80, button: 0 });
    expect(view.scrollLeft).toBeCloseTo(left + 20);
    expect(slot.getByTestId("selection-coordinates").textContent).toBe(
      coordinates,
    );
    expect((comment as HTMLTextAreaElement).value).toBe("Keep this comment");
    expect(fireEvent.keyDown(comment, { key: " ", code: "Space" })).toBe(true);
    fireEvent.keyDown(view, { key: " ", code: "Space" });
    fireEvent.blur(window);
    expect(view.className).not.toContain("grf-pan-ready");
    fireEvent.pointerDown(area, { clientX: 60, clientY: 40, button: 0 });
    fireEvent.pointerUp(area, { clientX: 80, clientY: 60, button: 0 });
    expect(slot.getByTestId("selection-coordinates").textContent).not.toBe(
      coordinates,
    );
  } finally {
    slot.unmount();
    vi.unstubAllGlobals();
  }
});

it("opens a file picker, handles cancellation, and uploads selected bytes in bounded requests", async () => {
  const { webcrypto } = await import("node:crypto");
  vi.stubGlobal("crypto", webcrypto);
  vi.stubGlobal(
    "ResizeObserver",
    class {
      observe() {}
      disconnect() {}
    },
  );
  const app = await loadPluginApp(() => import("./app"));
  const uploaded: string[] = [];
  const finish = vi.fn((input: unknown) => ({
    ...capture,
    title: "Picked image",
    sourcePath: "Выбранный файл: desktop.png",
  }));
  const slot = renderSlot(
    app.threadPanelActions[0]!,
    { threadId: "thread-1", params: { section: "review" } },
    {
      rpc: {
        list: () => [],
        get: () => ({
          capture: { ...capture, title: "Picked image" },
          annotations: [],
        }),
        image: () => ({
          mimeType: "image/png",
          data: "aGVsbG8=",
          nextOffset: 5,
          done: true,
        }),
        uploadChunk: (input) => {
          uploaded.push((input as { data: string }).data);
          return { received: 1 };
        },
        importUpload: finish,
      },
    },
  );
  try {
    const picker = slot.getByLabelText("Выбор снимка") as HTMLInputElement;
    const click = vi.spyOn(picker, "click");
    fireEvent.click(slot.getByText("Выбрать файл…"));
    expect(click).toHaveBeenCalledOnce();
    expect(picker.accept).toContain("image/png");
    fireEvent.change(picker, { target: { files: [] } });
    expect(uploaded).toHaveLength(0);
    const source = new Uint8Array(384 * 1024 + 50).fill(17);
    const file = new File([source], "desktop.png", { type: "image/png" });
    Object.defineProperty(file, "arrayBuffer", {
      value: async () => source.buffer,
    });
    fireEvent.change(picker, { target: { files: [file] } });
    await slot.findByAltText("Picked image");
    expect(uploaded).toHaveLength(2);
    expect(Buffer.from(uploaded.join(""), "base64")).toEqual(
      Buffer.from(source),
    );
    expect(finish).toHaveBeenCalledWith(
      expect.objectContaining({
        fileName: "desktop.png",
        totalBytes: source.length,
        threadId: "thread-1",
        title: "desktop.png",
        checksum: expect.stringMatching(/^[a-f0-9]{64}$/),
      }),
    );
    expect(picker.value).toBe("");
  } finally {
    slot.unmount();
    vi.unstubAllGlobals();
  }
});

it("browses GRF assets, plays preloaded ACT frames and opens the chosen frame for review", async () => {
  vi.stubGlobal(
    "ResizeObserver",
    class {
      observe() {}
      disconnect() {}
    },
  );
  let tick: FrameRequestCallback | undefined;
  vi.stubGlobal(
    "requestAnimationFrame",
    vi.fn((cb: FrameRequestCallback) => {
      tick = cb;
      return 1;
    }),
  );
  vi.stubGlobal("cancelAnimationFrame", vi.fn());
  const clock = vi.spyOn(performance, "now").mockReturnValue(0);
  const entry = {
    id: "c".repeat(64),
    path: "data/sprite/npc/rocker.act",
    type: "act",
    archive: 0,
    source: "data.grf",
    bytes: 500,
    variants: [0],
  };
  const info = {
    entry,
    version: "2.5",
    imageCount: 0,
    actions: [{ index: 0, name: "Idle", frames: 2, intervalMs: 100 }],
    pair: null,
    warnings: [],
  };
  const preview = {
    previewId: "33333333-3333-4333-8333-333333333333",
    info,
    fingerprint: "d".repeat(64),
    dependencies: [],
    renderer: "midgard-ro",
    action: 0,
    firstFrame: 0,
    frameCount: 2,
    width: 2,
    height: 2,
    columns: 2,
    originX: 1,
    originY: 2,
    intervalMs: 100,
    colorKey: true,
    image: { ...capture.image, width: 4, height: 2 },
    rendererVersion: "f".repeat(64),
  };
  const freeze = vi.fn(() => capture),
    render = vi.fn(() => preview);
  const app = await loadPluginApp(() => import("./app"));
  const slot = renderSlot(
    app.threadPanelActions[0]!,
    { threadId: "thread-1", params: null },
    {
      rpc: {
        grfSearch: () => ({
          archives: [{ index: 0, path: "data.grf", files: 1, bytes: 500 }],
          fingerprint: "d".repeat(64),
          total: 1,
          matched: 1,
          entries: [entry],
          next: -1,
        }),
        grfInspect: () => info,
        grfRender: render,
        grfImage: () => ({
          mimeType: "image/png",
          data: "aGVsbG8=",
          nextOffset: 5,
          done: true,
        }),
        grfCapture: freeze,
        list: () => [capture],
        get: () => ({ capture, annotations: [] }),
        image: () => ({
          mimeType: "image/png",
          data: "aGVsbG8=",
          nextOffset: 5,
          done: true,
        }),
      },
    },
  );
  try {
    fireEvent.click(await slot.findByRole("button", { name: /rocker.act/ }));
    const img = await slot.findByRole("img");
    vi.stubGlobal(
      "PointerEvent",
      class extends MouseEvent {
        pointerId = 1;
      },
    );
    const view = slot.getByLabelText("Просмотр ресурса GRF");
    view.setPointerCapture = vi.fn();
    view.hasPointerCapture = () => true;
    view.releasePointerCapture = vi.fn();
    view.scrollLeft = 100;
    view.scrollTop = 80;
    fireEvent.pointerEnter(view);
    expect(fireEvent.keyDown(view, { key: " ", code: "Space" })).toBe(false);
    expect(view.className).toContain("grf-pan-ready");
    fireEvent.pointerDown(img, { clientX: 50, clientY: 40, button: 0 });
    // Releasing Space mid-gesture keeps dragging until pointer up.
    fireEvent.keyUp(window, { key: " ", code: "Space" });
    fireEvent.pointerMove(view, { clientX: 80, clientY: 60, button: 0 });
    fireEvent.pointerUp(view, { clientX: 80, clientY: 60, button: 0 });
    expect(view.scrollLeft).toBe(70);
    expect(view.scrollTop).toBe(60);
    expect(view.className).not.toContain("grf-panning");
    fireEvent.pointerDown(img, { clientX: 50, clientY: 40, button: 0 });
    fireEvent.pointerUp(view, { clientX: 80, clientY: 60, button: 0 });
    expect(view.scrollLeft).toBe(70); // Ordinary clicks do not start a pan.
    fireEvent.pointerDown(img, { clientX: 50, clientY: 40, button: 1 });
    fireEvent.pointerUp(view, { clientX: 40, clientY: 30, button: 1 });
    expect(view.scrollLeft).toBe(80);
    expect(view.scrollTop).toBe(70);
    expect(
      fireEvent.keyDown(slot.getByLabelText("Поиск в GRF"), {
        key: " ",
        code: "Space",
      }),
    ).toBe(true);
    fireEvent.keyDown(view, { key: " ", code: "Space" });
    fireEvent.pointerDown(img, { clientX: 50, clientY: 40, button: 0 });
    fireEvent.blur(window);
    fireEvent.pointerMove(view, { clientX: 200, clientY: 200, button: 0 });
    expect(view.scrollLeft).toBe(80);
    expect(view.className).not.toContain("grf-pan-ready");
    fireEvent.click(slot.getByText("Воспроизвести"));
    const { act } = await import("@testing-library/react");
    await act(async () => {
      tick?.(150);
    });
    expect(
      (slot.getByLabelText("Кадр анимации") as HTMLInputElement).value,
    ).toBe("1");
    expect(img.style.backgroundPosition).toBe("-4px 0px");
    expect(render).toHaveBeenCalledOnce();
    fireEvent.click(slot.getByText("Выделить область и оставить замечание"));
    await slot.findByAltText("Wall screenshot");
    expect(freeze).toHaveBeenCalledWith({
      previewId: preview.previewId,
      frame: 1,
    });
  } finally {
    slot.unmount();
    clock.mockRestore();
    vi.unstubAllGlobals();
  }
});

it("selects an image region, edits and checks it, and removes it with trash or Delete without stealing typing", async () => {
  vi.stubGlobal(
    "ResizeObserver",
    class {
      observe() {}
      disconnect() {}
    },
  );
  let record: Annotation = {
    id: "22222222-2222-4222-8222-222222222222",
    captureId: capture.id,
    imageDigest: capture.image.digest,
    rect: { x: 20, y: 20, width: 40, height: 30 },
    comment: "Rocker region",
    createdAt: "2026-09-07",
    resolved: false,
    revision: 0,
    crop: { ...capture.image, width: 40, height: 30 },
  };
  const remove = vi.fn((input: unknown) => {
    expect(input).toEqual({
      annotationId: record.id,
      revision: record.revision,
    });
    record = {
      ...record,
      deletedAt: "2026-09-07",
      revision: record.revision + 1,
    };
    return record;
  });
  const restore = vi.fn(() => {
    record = { ...record, deletedAt: null, revision: record.revision + 1 };
    return record;
  });
  const edit = vi.fn((input: unknown) => {
    const p = input as {
      comment: string;
      resolved?: boolean;
      revision: number;
    };
    expect(p.revision).toBe(record.revision);
    record = {
      ...record,
      comment: p.comment,
      ...(p.resolved === undefined ? {} : { resolved: p.resolved }),
      revision: record.revision + 1,
    };
    return record;
  });
  const app = await loadPluginApp(() => import("./app"));
  const slot = renderSlot(
    app.threadPanelActions[0]!,
    { threadId: "thread-1", params: { captureId: capture.id } },
    {
      rpc: {
        list: () => [capture],
        get: () => ({ capture, annotations: record.deletedAt ? [] : [record] }),
        image: () => ({
          mimeType: "image/png",
          data: "aGVsbG8=",
          nextOffset: 5,
          done: true,
        }),
        updateAnnotation: edit,
        deleteAnnotation: remove,
        restoreAnnotation: restore,
      },
    },
  );
  try {
    await slot.findByAltText("Wall screenshot");
    let region = slot.getByRole("button", { name: "Выбрать область 1" });
    expect(region.getAttribute("aria-pressed")).toBe("false");
    fireEvent.click(region);
    expect(region.getAttribute("aria-pressed")).toBe("true");
    expect(region.className).toContain("grf-region-active");
    expect(
      slot
        .getByRole("button", { name: "Показать замечание 1" })
        .getAttribute("aria-pressed"),
    ).toBe("true");
    const comment = slot.getByLabelText("Комментарий выбранной области");
    fireEvent.change(comment, { target: { value: "This part is checked" } });
    expect(fireEvent.keyDown(comment, { key: "Delete" })).toBe(true);
    expect(fireEvent.keyDown(comment, { key: "Backspace" })).toBe(true);
    expect(remove).not.toHaveBeenCalled();
    fireEvent.click(slot.getByRole("button", { name: "Проверено" }));
    await slot.findByRole("button", { name: "Переоткрыть" });
    expect(edit).toHaveBeenCalledWith({
      annotationId: record.id,
      revision: 0,
      comment: "This part is checked",
      resolved: true,
    });
    fireEvent.click(slot.getByRole("button", { name: "Удалить область 1" }));
    await waitFor(() =>
      expect(
        slot.queryByRole("button", { name: "Выбрать область 1" }),
      ).toBeNull(),
    );
    expect(remove).toHaveBeenCalledOnce();
    expect(slot.getByAltText("Wall screenshot")).toBeTruthy();
    fireEvent.click(slot.getByRole("button", { name: "Вернуть" }));
    region = await slot.findByRole("button", { name: "Выбрать область 1" });
    fireEvent.click(region);
    const view = slot.getByLabelText("Просмотр снимка");
    expect(fireEvent.keyDown(view, { key: "Delete", repeat: true })).toBe(true);
    expect(fireEvent.keyDown(view, { key: "Delete", metaKey: true })).toBe(
      true,
    );
    expect(remove).toHaveBeenCalledOnce();
    expect(fireEvent.keyDown(view, { key: "Delete" })).toBe(false);
    await waitFor(() => expect(remove).toHaveBeenCalledTimes(2));
    await waitFor(() =>
      expect(
        slot.queryByRole("button", { name: "Выбрать область 1" }),
      ).toBeNull(),
    );
  } finally {
    slot.unmount();
    vi.unstubAllGlobals();
  }
});

it("resizes then moves a region as one draft, applies exact source pixels and supports Escape", async () => {
  vi.stubGlobal(
    "ResizeObserver",
    class {
      observe() {}
      disconnect() {}
    },
  );
  vi.stubGlobal(
    "PointerEvent",
    class extends MouseEvent {
      pointerId = 1;
    },
  );
  let record: Annotation = {
    id: "22222222-2222-4222-8222-222222222222",
    captureId: capture.id,
    imageDigest: capture.image.digest,
    rect: { x: 20, y: 20, width: 40, height: 30 },
    comment: "Keep this",
    createdAt: "2026-09-07",
    resolved: false,
    revision: 0,
    crop: { ...capture.image, width: 40, height: 30 },
  };
  const transform = vi.fn((input: unknown) => {
    const p = input as { rect: Annotation["rect"]; revision: number };
    record = {
      ...record,
      rect: p.rect,
      revision: p.revision + 1,
      crop: {
        ...record.crop,
        digest: "c".repeat(64),
        width: p.rect.width,
        height: p.rect.height,
      },
    };
    return record;
  });
  const image = vi.fn((_input: unknown) => ({
    mimeType: "image/png",
    data: "aGVsbG8=",
    nextOffset: 5,
    done: true,
  }));
  const app = await loadPluginApp(() => import("./app"));
  const slot = renderSlot(
    app.threadPanelActions[0]!,
    { threadId: "thread-1", params: { captureId: capture.id } },
    {
      rpc: {
        list: () => [capture],
        get: () => ({ capture, annotations: [record] }),
        image,
        transformAnnotation: transform,
      },
    },
  );
  try {
    const img = await slot.findByAltText("Wall screenshot");
    vi.spyOn(img, "getBoundingClientRect").mockReturnValue({
      left: 10,
      top: 10,
      width: 100,
      height: 50,
      right: 110,
      bottom: 60,
      x: 10,
      y: 10,
      toJSON: () => ({}),
    });
    const view = slot.getByLabelText("Просмотр снимка");
    view.setPointerCapture = vi.fn();
    view.hasPointerCapture = () => true;
    view.releasePointerCapture = vi.fn();
    fireEvent.click(slot.getByRole("button", { name: "Выбрать область 1" }));
    const editor = slot.getByLabelText("Комментарий выбранной области");
    expect(fireEvent.keyDown(editor, { key: "r", code: "KeyR" })).toBe(true);
    expect(
      slot.queryByLabelText("Изменить размер: правый нижний угол"),
    ).toBeNull();
    fireEvent.keyDown(view, { key: "r", code: "KeyR" });
    const handle = slot.getByLabelText("Изменить размер: правый нижний угол");
    expect(handle.style.cursor).toBe("nwse-resize");
    fireEvent.pointerDown(handle, { clientX: 40, clientY: 35, button: 0 });
    expect(view.style.cursor).toBe("nwse-resize");
    fireEvent.pointerMove(view, { clientX: 50, clientY: 40, button: 0 });
    fireEvent.pointerUp(view, { clientX: 50, clientY: 40, button: 0 });
    expect(
      slot.getByRole("button", { name: "Выбрать область 1" }).style.width,
    ).toBe("30%");
    expect(transform).not.toHaveBeenCalled();
    fireEvent.keyDown(view, { key: "g", code: "KeyG" });
    expect(view.className).toContain("grf-mode-move");
    expect(
      slot.queryByLabelText("Изменить размер: правый нижний угол"),
    ).toBeNull();
    fireEvent.pointerDown(
      slot.getByRole("button", { name: "Выбрать область 1" }),
      { clientX: 30, clientY: 30, button: 0 },
    );
    expect(view.style.cursor).toBe("move");
    fireEvent.pointerUp(view, { clientX: 35, clientY: 35, button: 0 });
    expect(transform).not.toHaveBeenCalled();
    fireEvent.keyDown(view, { key: "Enter" });
    await waitFor(() =>
      expect(transform).toHaveBeenCalledWith({
        annotationId: record.id,
        revision: 0,
        imageDigest: capture.image.digest,
        rect: { x: 30, y: 30, width: 60, height: 40 },
      }),
    );
    await waitFor(() =>
      expect(
        slot.queryByRole("button", { name: "Применить (Enter)" }),
      ).toBeNull(),
    );
    await waitFor(() =>
      expect(
        image.mock.calls.filter(
          (args) =>
            (args[0] as unknown as { annotationId?: string })?.annotationId,
        ).length,
      ).toBeGreaterThanOrEqual(2),
    );
    fireEvent.keyDown(view, { key: "g", code: "KeyG" });
    fireEvent.pointerDown(
      slot.getByRole("button", { name: "Выбрать область 1" }),
      { clientX: 35, clientY: 35, button: 0 },
    );
    fireEvent.pointerUp(view, { clientX: 45, clientY: 40, button: 0 });
    fireEvent.keyDown(view, { key: "r", code: "KeyR" });
    fireEvent.keyDown(view, { key: "Escape" });
    expect(transform).toHaveBeenCalledOnce();
    expect(
      slot.getByRole("button", { name: "Выбрать область 1" }).style.left,
    ).toBe("15%");
    expect(
      slot.getByRole("button", { name: "Выбрать область 1" }).style.width,
    ).toBe("30%");
    expect(
      (
        slot.getByLabelText(
          "Комментарий выбранной области",
        ) as HTMLTextAreaElement
      ).value,
    ).toBe("Keep this");
  } finally {
    slot.unmount();
    vi.unstubAllGlobals();
  }
});

it("confirms capture deletion, keeps Cancel safe and removes the selected snapshot from review", async () => {
  vi.stubGlobal(
    "ResizeObserver",
    class {
      observe() {}
      unobserve() {}
      disconnect() {}
    },
  );
  const app = await loadPluginApp(() => import("./app"));
  let present = true;
  const remove = vi.fn(() => {
    present = false;
    return {
      captureId: capture.id,
      threadId: capture.threadId,
      deletedAnnotations: 0,
    };
  });
  const slot = renderSlot(
    app.threadPanelActions[0]!,
    { threadId: capture.threadId, params: { captureId: capture.id } },
    {
      context: { threadId: capture.threadId, projectId: capture.projectId },
      rpc: {
        list: () => (present ? [capture] : []),
        get: () => {
          if (!present) throw new Error("Снимок не найден.");
          return { capture, annotations: [] };
        },
        image: () => ({
          mimeType: "image/png",
          data: "aGVsbG8=",
          nextOffset: 0,
          done: true,
        }),
        deleteCapture: remove,
      },
    },
  );
  try {
    await slot.findByAltText(capture.title);
    fireEvent.click(slot.getByRole("button", { name: "Удалить снимок" }));
    await slot.findByRole("alertdialog");
    expect(remove).not.toHaveBeenCalled();
    fireEvent.click(slot.getByRole("button", { name: "Отмена" }));
    await waitFor(() => expect(slot.queryByRole("alertdialog")).toBeNull());
    expect(remove).not.toHaveBeenCalled();
    fireEvent.click(slot.getByRole("button", { name: "Удалить снимок" }));
    await slot.findByRole("alertdialog");
    fireEvent.click(
      slot.getByRole("button", { name: "Удалить снимок и замечания" }),
    );
    await waitFor(() => expect(remove).toHaveBeenCalledTimes(1));
    await waitFor(() => expect(slot.queryByAltText(capture.title)).toBeNull());
    expect(
      slot.inspection.rpcCalls.find((c) => c.method === "deleteCapture")?.input,
    ).toEqual({ captureId: capture.id, imageDigest: capture.image.digest });
    fireEvent.click(slot.getByText("Ресурсы GRF"));
    fireEvent.click(slot.getByText("Снимки и замечания"));
    await waitFor(() => expect(slot.queryByAltText(capture.title)).toBeNull());
  } finally {
    slot.lifecycle.unmount();
    vi.unstubAllGlobals();
  }
});
