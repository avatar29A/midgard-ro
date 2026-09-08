import { z } from "zod";
export const libraryQuery = z
  .object({
    query: z.string().max(300).default(""),
    kind: z
      .enum(["", "procedural", "composite", "binding", "str", "act", "spr"])
      .default(""),
    mode: z
      .enum(["all", "curated", "bindings", "resources"])
      .default("curated"),
    skillId: z.number().int().min(0).max(65535).default(0),
    offset: z.number().int().min(0).max(1000000).default(0),
  })
  .strict();
export const libraryEntry = z
  .object({
    id: z.string(),
    name: z.string(),
    english: z.string(),
    description: z.string(),
    kind: z.string(),
    tags: z.array(z.string()),
    effects: z.array(z.string()),
    sources: z.array(z.string()),
    curated: z.boolean(),
    previewSkill: z.number().int(),
    skills: z.array(
      z
        .object({ id: z.number().int(), name: z.string(), phase: z.string() })
        .strict(),
    ),
    resources: z.array(
      z
        .object({
          id: z.string(),
          path: z.string(),
          type: z.string(),
          available: z.boolean(),
        })
        .strict(),
    ),
  })
  .strict();
export const effectLibrarySchema = z
  .object({
    entries: z.array(libraryEntry).max(50),
    total: z.number().int(),
    matched: z.number().int(),
    next: z.number().int(),
    revision: z.string(),
    fingerprint: z.string(),
    sourceVersion: z.string(),
  })
  .strict();
export type LibraryEntry = z.infer<typeof libraryEntry>;
export type EffectLibraryData = z.infer<typeof effectLibrarySchema>;
