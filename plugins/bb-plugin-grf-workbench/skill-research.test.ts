import { expect, it, vi } from "vitest";
import {
  createFakePluginHost,
  makeThreadResponse,
} from "@get-bb/plugin-sdk/testing";
import plugin from "./server";
const request = {
  projectId: "chosen-project",
  providerId: "codex",
  model: "chosen",
  reasoningLevel: "high",
  permissionMode: "auto",
  executionInputSources: { model: "explicit", providerId: "explicit" },
  environment: { type: "reuse", environmentId: "chosen-env" },
  input: [{ type: "text", text: "My edited research prompt", mentions: [] }],
};
it("forwards the chosen composer request and does not duplicate successful or uncertain creates", async () => {
  const spawn = vi.fn(async (_input: unknown) =>
    makeThreadResponse({ id: "new-research" }),
  );
  const host = createFakePluginHost({
    pluginId: "grf-workbench",
    sdk: {
      threads: {
        get: async () =>
          makeThreadResponse({
            id: "source",
            projectId: "source-project",
            environmentId: "source-env",
          }),
        spawn,
      },
    },
  });
  try {
    // Existing 0.9.0 history must remain an exact prefix on upgrade.
    host.bb.storage.migrate(host.bb.storage.database(), [
      "CREATE TABLE captures (id TEXT PRIMARY KEY, thread_id TEXT NOT NULL, project_id TEXT NOT NULL, created_at TEXT NOT NULL, payload TEXT NOT NULL)",
      "CREATE TABLE annotations (id TEXT PRIMARY KEY, capture_id TEXT NOT NULL REFERENCES captures(id), created_at TEXT NOT NULL, payload TEXT NOT NULL)",
      "CREATE INDEX captures_thread ON captures(thread_id, created_at)",
      "CREATE INDEX annotations_capture ON annotations(capture_id, created_at)",
      "CREATE TABLE grf_previews (id TEXT PRIMARY KEY, payload TEXT NOT NULL)",
      "CREATE TABLE review_launches (id TEXT PRIMARY KEY, thread_id TEXT, payload TEXT NOT NULL)",
      "CREATE INDEX review_launches_thread ON review_launches(thread_id)",
    ]);
    await plugin(host.bb);
    expect(
      await host.harness.behavior.callRpc("researchContext", {
        threadId: "source",
      }),
    ).toEqual({ projectId: "source-project", environmentId: "source-env" });
    const input = {
      operationId: "11111111-1111-4111-8111-111111111111",
      sourceThreadId: "source",
      skillId: 89,
      skillName: "Storm Gust",
      request,
    };
    expect(await host.harness.behavior.callRpc("startResearch", input)).toEqual(
      { threadId: "new-research" },
    );
    expect(spawn.mock.calls[0]?.[0]).toMatchObject({
      ...request,
      title: "Skill Studio · Storm Gust",
    });
    host.harness = (await host.harness.lifecycle.reload(plugin)).harness;
    await host.harness.behavior.callRpc("startResearch", input);
    expect(spawn).toHaveBeenCalledTimes(1);
    spawn.mockRejectedValueOnce(new Error("lost response"));
    const uncertain = {
      ...input,
      operationId: "22222222-2222-4222-8222-222222222222",
    };
    await expect(
      host.harness.behavior.callRpc("startResearch", uncertain),
    ).rejects.toThrow("lost response");
    await expect(
      host.harness.behavior.callRpc("startResearch", uncertain),
    ).rejects.toThrow("повторный тред автоматически не создаётся");
    expect(spawn).toHaveBeenCalledTimes(2);
  } finally {
    await host.harness.lifecycle.dispose();
  }
});
