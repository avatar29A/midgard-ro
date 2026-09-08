import { expect, it, vi } from "vitest";
import { crossedSounds, StudioSoundPlayer } from "./studio-audio";
import { frameFixture } from "./test-support/studio";
const events = [
  { id: "cast", kind: "sound", tick: 0, owner: "caster", sound: "cast.wav" },
  { id: "impact", kind: "sound", tick: 56, owner: "target", sound: "hit.wav" },
];
it("plays crossed sound events once, including a loop boundary", () => {
  expect(crossedSounds(events, 0, 0, 157)).toEqual([]);
  expect(crossedSounds(events, 55, 57, 157).map((e) => e.id)).toEqual([
    "impact",
  ]);
  expect(crossedSounds(events, 55, 1, 157).map((e) => e.id)).toEqual([
    "impact",
    "cast",
  ]);
});
it("stops on pause, stays silent during camera redraws, and does not repeat the paused boundary", async () => {
  const start = vi.fn(),
    stop = vi.fn(),
    disconnect = vi.fn();
  vi.stubGlobal(
    "AudioContext",
    class {
      destination = {};
      resume = async () => {};
      close = async () => {};
      decodeAudioData = async () => ({});
      createBufferSource = () => ({
        start,
        stop,
        disconnect,
        connect: () => {},
        playbackRate: { value: 1 },
      });
    },
  );
  const player = new StudioSoundPlayer();
  try {
    await player.unlock();
    await player.load([
      { path: "cast.wav", sha256: "a".repeat(64), data: "aGVsbG8=" },
    ]);
    const frame = frameFixture();
    frame.context.tick = 0;
    frame.context.timeline = {
      castMs: 500,
      releaseTick: 30,
      endTick: 127,
      durationTicks: 157,
      reactionTicks: [43],
      impactTicks: [56],
      cancelTick: null,
      events,
    };
    player.begin(0);
    player.advance(frame, 1);
    expect(start).toHaveBeenCalledTimes(1);
    player.advance(frame, 1);
    expect(start).toHaveBeenCalledTimes(1);
    player.stop();
    expect(stop).toHaveBeenCalledTimes(1);
    player.begin(0);
    player.advance(frame, 1);
    expect(start).toHaveBeenCalledTimes(1);
    player.stop();
    frame.context.tick = 56;
    player.advance(frame, 1);
    expect(start).toHaveBeenCalledTimes(1);
  } finally {
    await player.dispose();
    vi.unstubAllGlobals();
  }
});
