import { z } from "zod";
export const catalogSkill = z
  .object({
    id: z.number().int(),
    name: z.string(),
    icon: z.string(),
    jobs: z.array(z.number().int()),
    hasInfo: z.boolean(),
    kind: z.string(),
    target: z.string(),
    elements: z.array(z.string()),
    hits: z.array(z.number()),
    castMs: z.array(z.number()),
    fixedMs: z.array(z.number()),
    bindings: z.array(
      z
        .object({
          phase: z.enum(["cast", "caster", "target", "ground"]),
          effects: z.array(z.string()),
        })
        .strict(),
    ),
    studio: z.enum(["preview", "partial", "mapped", "unmapped"]),
    sources: z.array(z.string()),
  })
  .strict();
export const skillCatalogSchema = z
  .object({
    jobs: z
      .array(
        z
          .object({
            id: z.number().int(),
            name: z.string(),
            skills: z.array(z.number().int()),
          })
          .strict(),
      )
      .max(1000),
    skills: z.array(catalogSkill).max(10000),
    source: z.string(),
    sourceVersion: z.string().regex(/^[a-f0-9]{64}$/),
  })
  .strict();
export type SkillCatalogData = z.infer<typeof skillCatalogSchema>;
