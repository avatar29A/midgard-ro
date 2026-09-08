// @vitest-environment jsdom
import { expect, it, vi } from "vitest";
import { fireEvent, waitFor } from "@testing-library/react";
import { loadPluginApp, renderSlot } from "@get-bb/plugin-sdk/testing/app";
import { catalogFixture } from "./test-support/catalog";
import { frameFixture } from "./test-support/studio";
import type { StudioScene } from "./studio-contract";
it("opens the remaining Mage skills and preserves explicit target status inputs", async () => {
  HTMLImageElement.prototype.decode = async () => {};
  const catalog = structuredClone(catalogFixture);
  const added = [
    [11, "Napalm Beat"],
    [14, "Cold Bolt"],
    [15, "Frost Diver"],
    [16, "Stone Curse"],
    [17, "Fire Ball"],
    [19, "Fire Bolt"],
    [20, "Lightning Bolt"],
    [21, "Thunderstorm"],
  ] as const;
  for (const [id, name] of added) {
    catalog.jobs[0].skills.push(id);
    catalog.skills.push({
      ...catalog.skills[0],
      id,
      name,
      jobs: [2],
      studio: id === 16 ? "partial" : "preview",
    });
  }
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
  const app = await loadPluginApp(() => import("./app"));
  const slot = renderSlot(
    app.threadPanelActions[0]!,
    { threadId: "t1", params: { section: "studio" } },
    {
      context: { threadId: "t1", projectId: "p1" },
      rpc: {
        skillCatalog: () => catalog,
        studioRender: (i) => render(i as { scene: StudioScene }),
        studioClose: () => ({ closed: true }),
      },
    },
  );
  try {
    await slot.findByAltText("Soul Strike · tick 0 · 0°");
    for (const [id, name] of added) {
      fireEvent.change(slot.getByLabelText("Навык"), {
        target: { value: String(id) },
      });
      await slot.findByAltText(`${name} · tick 0 · 0°`);
      expect(render.mock.calls.at(-1)?.[0].scene.skillId).toBe(id);
      expect(!!slot.queryByLabelText("Входное число попаданий")).toBe(
        id !== 16 && id !== 21,
      );
      expect(!!slot.queryByLabelText("Положение эффекта")).toBe(id === 21);
      if (id === 15 || id === 16) {
        fireEvent.change(slot.getByLabelText("Состояние цели"), {
          target: { value: id === 15 ? "frozen" : "stone" },
        });
        fireEvent.change(slot.getByLabelText("Задержка состояния"), {
          target: { value: "750" },
        });
        fireEvent.change(slot.getByLabelText("Время жизни эффекта"), {
          target: { value: "1200" },
        });
        await waitFor(() =>
          expect(render.mock.calls.at(-1)?.[0].scene).toMatchObject({
            targetStatus: id === 15 ? "frozen" : "stone",
            statusDelayMs: 750,
            effectDurationMs: 1200,
          }),
        );
      }
    }
  } finally {
    slot.lifecycle.unmount();
  }
});
