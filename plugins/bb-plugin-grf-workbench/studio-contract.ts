import { z } from "zod";

const hash = z.string().regex(/^[a-f0-9]{64}$/);
export const studioScene = z
  .object({
    tick: z.number().int().min(0).max(600),
    level: z.number().int().min(1).max(10),
    hits: z.number().int().min(0).max(20),
    separation: z.number().min(1).max(30),
    camera: z
      .object({
        yaw: z.number().min(-360).max(360),
        distance: z.number().min(100).max(800),
        panX: z.number().min(-200).max(200),
        panZ: z.number().min(-200).max(200),
        focus: z.enum(["center", "caster", "target"]),
      })
      .strict(),
  })
  .strict();
export const studioContext = z
  .object({
    schemaVersion: z.literal(1),
    skillId: z.literal(13),
    effectId: z.literal("soul_strike.default"),
    generator: z.literal("procedural.soul_strike"),
    contractVersion: z.literal(1),
    phase: z.literal("volley"),
    tick: z.number().int(),
    timeMs: z.number(),
    tickRate: z.literal(60),
    scene: studioScene,
    sceneDigest: hash,
    definitionDigest: hash,
    definitionSource: z.string(),
    definition: z
      .object({ halfSize: z.number(), rise: z.number(), spread: z.number() })
      .passthrough(),
    hitCount: z.number().int(),
    projectileCount: z.number().int(),
    particleCount: z.number().int(),
    visibleQuads: z.number().int(),
    durationTicks: z.number().int(),
    impactTicks: z.array(z.number().int()),
    renderer: z.string(),
    limitations: z.array(z.string()),
    dependencies: z.array(
      z.object({ path: z.string(), source: z.string(), sha256: hash }).strict(),
    ),
  })
  .passthrough();
export const studioSnapshot = studioContext.extend({ rendererVersion: hash });
export const studioFrame = z
  .object({
    frameId: z.string().uuid(),
    png: z.string().max(6 * 1024 * 1024),
    context: studioSnapshot,
  })
  .strict();
export const studioScope = z
  .object({ threadId: z.string().min(1), sessionId: z.string().uuid() })
  .strict();
export type StudioScene = z.infer<typeof studioScene>;
export type StudioFrame = z.infer<typeof studioFrame>;
export const defaultStudioScene: StudioScene = {
  tick: 0,
  level: 10,
  hits: 0,
  separation: 12,
  camera: { yaw: 0, distance: 190, panX: 0, panZ: 0, focus: "center" },
};
