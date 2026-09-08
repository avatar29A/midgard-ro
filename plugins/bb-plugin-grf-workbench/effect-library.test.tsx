// @vitest-environment jsdom
import { expect, it, vi } from "vitest";
import { fireEvent, waitFor } from "@testing-library/react";
import { loadPluginApp, renderSlot } from "@get-bb/plugin-sdk/testing/app";
import { catalogFixture } from "./test-support/catalog";
import { frameFixture } from "./test-support/studio";
import type { StudioScene } from "./studio-contract";
import type { EffectLibraryData } from "./effect-library-contract";

it("finds semantic effects, opens procedural and standalone STR previews, and links back to a skill", async () => {
  HTMLImageElement.prototype.decode = async () => {};
  const id = "d".repeat(64),
    revision = "e".repeat(64);
  const common = {
    english: "",
    description: "Fixture",
    tags: [],
    effects: [],
    sources: [],
    curated: true,
    skills: [{ id: 13, name: "Soul Strike", phase: "target" }],
  };
  const data: EffectLibraryData = {
    total: 2,
    matched: 2,
    next: -1,
    revision,
    fingerprint: "f",
    sourceVersion: "a".repeat(64),
    entries: [
      {
        ...common,
        id: "soul_strike.default",
        name: "Удар души",
        kind: "procedural",
        previewSkill: 13,
        resources: [],
      },
      {
        ...common,
        id: "resource:" + id,
        name: "Анимация стены",
        kind: "str",
        previewSkill: 0,
        resources: [
          {
            id,
            path: "data/texture/effect/safetywall.str",
            type: "str",
            available: true,
          },
        ],
      },
    ],
  };
  const library = vi.fn((_input: unknown) => data);
  const render = vi.fn(async ({ scene }: { scene: StudioScene }) => {
    const f = frameFixture();
    return {
      ...f,
      context: {
        ...f.context,
        scene,
        tick: scene.tick,
        skillId: scene.strResourceId ? 0 : 13,
        skillName: scene.strResourceId ? "safetywall.str" : "Soul Strike",
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
        skillCatalog: () => catalogFixture,
        effectLibrary: library,
        studioRender: (i) => render(i as { scene: StudioScene }),
        studioClose: () => ({ closed: true }),
      },
    },
  );
  try {
    await slot.findByAltText("Soul Strike · tick 0 · 0°");
    fireEvent.click(slot.getByRole("button", { name: "Эффекты и анимации" }));
    await slot.findByRole("option", { name: "Удар души · Процедурный" });
    fireEvent.change(slot.getByLabelText("Поиск эффекта"), {
      target: { value: "душа" },
    });
    await waitFor(() =>
      expect(library.mock.calls.at(-1)?.[0]).toMatchObject({ query: "душа" }),
    );
    fireEvent.change(slot.getByLabelText("Карточка эффекта"), {
      target: { value: "soul_strike.default" },
    });
    fireEvent.click(
      slot.getByRole("button", { name: "Показать эффект в сцене" }),
    );
    await waitFor(() =>
      expect(render.mock.calls.at(-1)?.[0].scene).toMatchObject({
        libraryEntryId: "soul_strike.default",
        libraryRevision: revision,
      }),
    );
    expect(render.mock.calls.at(-1)?.[0].scene.sequence).toBeUndefined();
    fireEvent.change(slot.getByLabelText("Карточка эффекта"), {
      target: { value: "resource:" + id },
    });
    fireEvent.click(slot.getByRole("button", { name: "Открыть STR" }));
    await slot.findByAltText("safetywall.str · tick 0 · 0°");
    expect(render.mock.calls.at(-1)?.[0].scene.strResourceId).toBe(id);
    expect(slot.queryByLabelText("Последовательность навыка")).toBeNull();
    expect(slot.queryByRole("button", { name: "Вращать" })).toBeNull();
    fireEvent.click(slot.getByText("Используется в навыках (1)"));
    fireEvent.click(slot.getByRole("button", { name: "Soul Strike · target" }));
    await slot.findByAltText("Soul Strike · tick 0 · 0°");
    expect(slot.getByLabelText("Навык")).toHaveProperty("value", "13");
    fireEvent.click(slot.getByRole("button", { name: "Эффекты в библиотеке" }));
    await waitFor(() =>
      expect(library.mock.calls.at(-1)?.[0]).toMatchObject({
        skillId: 13,
        mode: "all",
      }),
    );
  } finally {
    slot.lifecycle.unmount();
  }
});
