import { z } from "zod";

export const previewSkillId = z.union([
  z.literal(10),
  z.literal(11),
  z.literal(12),
  z.literal(13),
  z.literal(14),
  z.literal(15),
  z.literal(16),
  z.literal(17),
  z.literal(18),
  z.literal(19),
  z.literal(20),
  z.literal(21),
]);
export type PreviewSkillId = z.infer<typeof previewSkillId>;
export const hasTargetHits = (id: PreviewSkillId) =>
  [11, 13, 14, 15, 17, 19, 20].includes(id);
export const isPersistent = (id: PreviewSkillId) => [10, 12, 18].includes(id);
export const hasGround = (id: PreviewSkillId) => [12, 18, 21].includes(id);
const hash = z.string().regex(/^[a-f0-9]{64}$/);
export const studioScene = z
  .object({
    strResourceId: z
      .string()
      .regex(/^[a-f0-9]{64}$/)
      .optional(),
    libraryEntryId: z.string().max(200).optional(),
    libraryRevision: z
      .string()
      .regex(/^[a-f0-9]{64}$/)
      .optional(),
    tick: z.number().int().min(0).max(3600),
    skillId: previewSkillId.optional(),
    targetStatus: z.enum(["none", "frozen", "stone"]).optional(),
    statusDelayMs: z.number().int().min(0).max(10000).optional(),
    effectDurationMs: z.number().int().min(100).max(30000).optional(),
    ground: z
      .object({
        anchor: z.enum(["caster", "target"]),
        cells: z.union([z.literal(1), z.literal(3)]),
        angle: z.union([z.literal(0), z.literal(90)]),
      })
      .strict()
      .optional(),
    level: z.number().int().min(1).max(10),
    hits: z.number().int().min(0).max(20),
    separation: z.number().min(1).max(30),
    sequence: z
      .object({
        castMs: z.number().int().min(-1).max(6000),
        cancelTick: z.number().int().min(0).max(360).nullable(),
        reaction: z.boolean(),
      })
      .strict()
      .optional(),
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
export const studioEvent = z
  .object({
    id: z.string(),
    kind: z.string(),
    tick: z.number().int(),
    owner: z.string(),
    sound: z.string().optional(),
  })
  .strict();
export const studioTimeline = z
  .object({
    castMs: z.number().int(),
    releaseTick: z.number().int(),
    durationTicks: z.number().int(),
    endTick: z.number().int(),
    reactionTicks: z.array(z.number().int()),
    impactTicks: z.array(z.number().int()),
    events: z.array(studioEvent),
    cancelTick: z.number().int().nullable(),
  })
  .strict();
export const studioAudio = z
  .object({
    clips: z
      .array(
        z
          .object({
            path: z.string(),
            sha256: hash,
            mimeType: z.literal("audio/wav"),
            data: z.string().max(1500000),
          })
          .strict(),
      )
      .max(8),
    warnings: z.array(z.string()),
  })
  .strict();
export type StudioEvent = z.infer<typeof studioEvent>;
export const studioContext = z
  .object({
    schemaVersion: z.literal(1),
    skillId: previewSkillId.or(z.literal(0)),
    effectId: z.string(),
    generator: z.string(),
    skillName: z.string().optional(),
    contractVersion: z.literal(1),
    phase: z.enum(["cast", "volley", "tail", "finished", "canceled", "active"]),
    tick: z.number().int(),
    timeMs: z.number(),
    tickRate: z.literal(60),
    scene: studioScene,
    sceneDigest: hash,
    definitionDigest: hash,
    definitionSource: z.string(),
    definition: z
      .object({
        halfSize: z.number().optional(),
        rise: z.number().optional(),
        spread: z.number().optional(),
        parameters: z
          .record(z.string(), z.union([z.string(), z.number(), z.boolean()]))
          .optional(),
      })
      .passthrough(),
    hitCount: z.number().int(),
    projectileCount: z.number().int(),
    particleCount: z.number().int(),
    visibleQuads: z.number().int(),
    durationTicks: z.number().int(),
    impactTicks: z.array(z.number().int()),
    timeline: studioTimeline.optional(),
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
  sequence: { castMs: -1, cancelTick: null, reaction: true },
  camera: { yaw: 0, distance: 190, panX: 0, panZ: 0, focus: "center" },
};

export function sceneForSkill(skillId: PreviewSkillId): StudioScene {
  if (skillId === 13) return defaultStudioScene;
  return {
    ...defaultStudioScene,
    skillId,
    level: 1,
    sequence: {
      castMs: -1,
      cancelTick: null,
      reaction: hasTargetHits(skillId),
    },
    ...(isPersistent(skillId) || skillId === 15 || skillId === 16
      ? { effectDurationMs: 3000 }
      : {}),
    ...(skillId === 15 || skillId === 16
      ? { targetStatus: "none" as const, statusDelayMs: 500 }
      : {}),
    ...(hasGround(skillId)
      ? {
          ground: {
            anchor: "target" as const,
            cells: skillId === 18 ? (3 as const) : (1 as const),
            angle: 0 as const,
          },
        }
      : {}),
  };
}
