import { z } from "zod";
// Wire shape of BB 0.41's NewThreadRequest. The host composer remains the
// authority for selections; these schemas validate and preserve its payload.
const name = z.string().max(4096),
  label = { label: z.string().max(4096) };
const resource = z.discriminatedUnion("kind", [
  z.object({
    kind: z.literal("thread"),
    ...label,
    threadId: name,
    projectId: name.optional(),
  }),
  z.object({ kind: z.literal("project"), ...label, projectId: name }),
  z.object({ kind: z.literal("section"), ...label, sectionId: name }),
  z.object({
    kind: z.literal("path"),
    ...label,
    path: name,
    entryKind: z.enum(["file", "directory"]),
    source: z.enum(["workspace", "thread-storage"]),
  }),
  z.object({
    kind: z.literal("command"),
    ...label,
    name,
    argumentHint: z.string().nullable(),
    origin: z.enum(["builtin", "project", "user"]),
    source: z.enum(["command", "skill"]),
    trigger: z.literal("/"),
  }),
  z.object({
    kind: z.literal("plugin"),
    ...label,
    pluginId: name,
    itemId: name,
    icon: z.string().nullable().optional(),
  }),
]);
const visibility = z.literal("agent-only").optional();
const input = z.discriminatedUnion("type", [
  z
    .object({
      type: z.literal("text"),
      text: z.string().max(200000),
      mentions: z
        .array(z.object({ start: z.number(), end: z.number(), resource }))
        .max(200),
      visibility,
    })
    .strict(),
  z
    .object({
      type: z.literal("image"),
      url: z.string().max(8 * 1024 * 1024),
      visibility,
    })
    .strict(),
  z.object({ type: z.literal("localImage"), path: name, visibility }).strict(),
  z
    .object({
      type: z.literal("localFile"),
      path: name,
      name: name.optional(),
      mimeType: name.optional(),
      sizeBytes: z.number().optional(),
      visibility,
    })
    .strict(),
]);
export const reviewEnvironmentSchema = z.discriminatedUnion("type", [
  z.object({ type: z.literal("reuse"), environmentId: name }).strict(),
  z.object({ type: z.literal("project-default") }).strict(),
  z
    .object({
      type: z.literal("host"),
      hostId: name.optional(),
      workspace: z.discriminatedUnion("type", [
        z
          .object({
            type: z.literal("unmanaged"),
            path: name.nullable(),
            branch: z
              .discriminatedUnion("kind", [
                z.object({ kind: z.literal("existing"), name }),
                z.object({ kind: z.literal("new"), baseBranch: name }),
              ])
              .optional(),
          })
          .strict(),
        z
          .object({
            type: z.literal("managed-worktree"),
            baseBranch: z.discriminatedUnion("kind", [
              z.object({ kind: z.literal("default") }),
              z.object({ kind: z.literal("named"), name }),
            ]),
          })
          .strict(),
        z.object({ type: z.literal("personal") }).strict(),
      ]),
    })
    .strict(),
]);
const source = z.enum(["explicit", "client-preference"]).optional();
export const reviewRequestSchema = z
  .object({
    projectId: name,
    providerId: name,
    model: name,
    reasoningLevel: z.enum([
      "none",
      "low",
      "medium",
      "high",
      "xhigh",
      "max",
      "ultra",
      "ultracode",
    ]),
    permissionMode: z.enum(["accept-edits", "auto", "full"]),
    serviceTier: z.enum(["default", "fast"]).optional(),
    executionInputSources: z
      .object({
        providerId: source,
        model: source,
        reasoningLevel: source,
        permissionMode: source,
        serviceTier: source,
      })
      .strict(),
    environment: reviewEnvironmentSchema,
    input: z.array(input).min(1).max(50),
    sendAt: z.number().optional(),
  })
  .strict()
  .refine(
    (v) => JSON.stringify(v).length <= 8 * 1024 * 1024,
    "Запрос превышает 8 MiB",
  );
