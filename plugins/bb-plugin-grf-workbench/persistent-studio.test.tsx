// @vitest-environment jsdom
import { expect, it, vi } from "vitest";
import { fireEvent, waitFor } from "@testing-library/react";
import { loadPluginApp, renderSlot } from "@get-bb/plugin-sdk/testing/app";
import { catalogFixture } from "./test-support/catalog";
import { frameFixture } from "./test-support/studio";
import type { StudioScene } from "./studio-contract";
it("routes each persistent skill to its own renderer input and preserves ground/lifetime controls", async () => {
  HTMLImageElement.prototype.decode = async () => {};
  const catalog = structuredClone(catalogFixture);
  catalog.jobs[0].skills.push(12, 18);
  catalog.skills.find((s) => s.id === 10)!.studio = "preview";
  for (const [id, name] of [
    [12, "Safety Wall"],
    [18, "Fire Wall"],
  ] as const)
    catalog.skills.push({
      ...catalog.skills[0],
      id,
      name,
      studio: "preview",
      jobs: [2],
    });
  const render = vi.fn(async ({ scene }: { scene: StudioScene }) => {
    const f = frameFixture(),
      id = scene.skillId ?? 13;
    return {
      ...f,
      context: {
        ...f.context,
        skillId: id,
        skillName: catalog.skills.find((s) => s.id === id)!.name,
        scene,
        tick: scene.tick,
      },
    };
  });
  const close = vi.fn(() => ({ closed: true }));
  const app = await loadPluginApp(() => import("./app"));
  const slot = renderSlot(
    app.threadPanelActions[0]!,
    { threadId: "t1", params: { section: "studio" } },
    {
      context: { threadId: "t1", projectId: "p1" },
      rpc: {
        skillCatalog: () => catalog,
        studioRender: (i) => render(i as { scene: StudioScene }),
        studioClose: close,
      },
    },
  );
  try {
    await slot.findByAltText("Soul Strike · tick 0 · 0°");
    fireEvent.change(slot.getByLabelText("Навык"), { target: { value: "10" } });
    await slot.findByAltText("Sight · tick 0 · 0°");
    expect(render.mock.calls.at(-1)?.[0].scene.skillId).toBe(10);
    expect(slot.queryByLabelText("Входное число попаданий")).toBeNull();
    fireEvent.change(slot.getByLabelText("Время жизни эффекта"), {
      target: { value: "1200" },
    });
    await waitFor(() =>
      expect(render.mock.calls.at(-1)?.[0].scene.effectDurationMs).toBe(1200),
    );
    fireEvent.change(slot.getByLabelText("Навык"), { target: { value: "18" } });
    await slot.findByAltText("Fire Wall · tick 0 · 0°");
    expect(render.mock.calls.at(-1)?.[0].scene.ground?.cells).toBe(3);
    fireEvent.change(slot.getByLabelText("Клетки Fire Wall"), {
      target: { value: "1" },
    });
    fireEvent.change(slot.getByLabelText("Положение эффекта"), {
      target: { value: "caster" },
    });
    await waitFor(() =>
      expect(render.mock.calls.at(-1)?.[0].scene.ground).toMatchObject({
        cells: 1,
        anchor: "caster",
      }),
    );
    fireEvent.change(slot.getByLabelText("Навык"), { target: { value: "12" } });
    await slot.findByAltText("Safety Wall · tick 0 · 0°");
    expect(slot.queryByLabelText("Клетки Fire Wall")).toBeNull();
    expect(render.mock.calls.at(-1)?.[0].scene.ground?.cells).toBe(1);
    expect(close.mock.calls.length).toBeGreaterThanOrEqual(3);
  } finally {
    slot.lifecycle.unmount();
  }
});
