// @vitest-environment jsdom
import { expect, it, vi } from "vitest";
import { fireEvent, waitFor } from "@testing-library/react";
import { loadPluginApp, renderSlot } from "@get-bb/plugin-sdk/testing/app";
import { catalogFixture } from "./test-support/catalog";
import { frameFixture } from "./test-support/studio";
it("opens the playable skill, stops it when inspecting an unsupported skill and seeds research with its ID", async () => {
  HTMLImageElement.prototype.decode = async () => {};
  const app = await loadPluginApp(() => import("./app")),
    close = vi.fn(() => ({ closed: true }));
  const render = vi.fn(() => frameFixture());
  const slot = renderSlot(
    app.threadPanelActions[0]!,
    { threadId: "thread-1", params: { section: "studio" } },
    {
      context: { threadId: "thread-1", projectId: "project-1" },
      rpc: {
        skillCatalog: () => catalogFixture,
        researchContext: () => ({
          projectId: "project-1",
          environmentId: "env-1",
        }),
        studioRender: render,
        studioClose: close,
      },
    },
  );
  try {
    await slot.findByAltText("Soul Strike · tick 26 · 0°");
    fireEvent.change(slot.getByLabelText("Навык"), { target: { value: "10" } });
    await slot.findByText("На кастере: EF_SIGHT");
    await waitFor(() => expect(close).toHaveBeenCalled());
    expect(slot.queryByAltText("Soul Strike · tick 26 · 0°")).toBeNull();
    fireEvent.click(slot.getByText("Подготовить исследование в новом треде"));
    const first = await slot.findByTestId("bb-new-thread-composer");
    const firstKey = first.getAttribute("data-draft-key");
    expect(firstKey).toBe("grf-skill-research:thread-1:10");
    expect(
      (slot.getByTestId("bb-new-thread-composer-input") as HTMLTextAreaElement)
        .value,
    ).toContain("Sight (skill ID 10)");
    fireEvent.change(slot.getByTestId("bb-new-thread-composer-input"), {
      target: { value: "Holy Cross old draft" },
    });
    fireEvent.change(slot.getByLabelText("Профессия"), {
      target: { value: "9" },
    });
    await slot.findByText("На земле: EF_STORMGUST");
    expect(render).toHaveBeenCalledTimes(1);
    fireEvent.click(slot.getByText("Подготовить исследование в новом треде"));
    const second = await slot.findByTestId("bb-new-thread-composer");
    expect(second.getAttribute("data-draft-key")).toBe(
      "grf-skill-research:thread-1:89",
    );
    expect(second.getAttribute("data-draft-key")).not.toBe(firstKey);
    const prompt = (
      slot.getByTestId("bb-new-thread-composer-input") as HTMLTextAreaElement
    ).value;
    expect(prompt).toContain("Storm Gust (skill ID 89)");
    expect(prompt).toContain("EF_STORMGUST");
    expect(prompt).not.toContain("Holy Cross");
    expect(
      slot.inspection.navigateCalls.some((c) => c.method === "toCompose"),
    ).toBe(false);

    fireEvent.change(slot.getByLabelText("Поиск навыка"), {
      target: { value: "missing-skill" },
    });
    await slot.findByText("Нет навыков по выбранным условиям.");
  } finally {
    slot.lifecycle.unmount();
  }
});
