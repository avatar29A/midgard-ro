import {
  defaultStudioScene,
  studioSnapshot,
  type StudioFrame,
} from "../studio-contract";
export function frameFixture(): StudioFrame {
  return {
    frameId: "33333333-3333-4333-8333-333333333333",
    png: "aGVsbG8=",
    context: studioSnapshot.parse({
      schemaVersion: 1,
      skillId: 13,
      effectId: "soul_strike.default",
      generator: "procedural.soul_strike",
      contractVersion: 1,
      phase: "volley",
      tick: 26,
      timeMs: 433.333,
      tickRate: 60,
      scene: { ...defaultStudioScene, tick: 26 },
      sceneDigest: "a".repeat(64),
      definitionDigest: "b".repeat(64),
      definitionSource: "definitions/soul_strike.yaml",
      definition: { halfSize: 18, rise: 16, spread: 20 },
      hitCount: 5,
      projectileCount: 5,
      particleCount: 35,
      visibleQuads: 12,
      durationTicks: 97,
      impactTicks: [26, 40, 54, 68, 82],
      renderer: "midgard-ro",
      rendererVersion: "c".repeat(64),
      limitations: ["volley only"],
      dependencies: [
        { path: "particle1.spr", source: "data.grf", sha256: "d".repeat(64) },
      ],
    }),
  };
}
