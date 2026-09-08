import type { StudioEvent, StudioFrame } from "./studio-contract";

// Events are triggered only while advancing displayed playback frames. A seek
// establishes a new cursor and never replays the skipped sounds.
export function crossedSounds(
  events: StudioEvent[],
  from: number,
  to: number,
  end: number,
) {
  const sound = events.filter((e) => e.kind === "sound" && e.sound);
  return to >= from
    ? sound.filter((e) => e.tick > from && e.tick <= to)
    : [
        ...sound.filter((e) => e.tick > from && e.tick <= end),
        ...sound.filter((e) => e.tick <= to),
      ];
}
export class StudioSoundPlayer {
  private ctx?: AudioContext;
  private clips = new Map<string, { buffer: AudioBuffer; sha256: string }>();
  private active = new Set<AudioBufferSourceNode>();
  private cursor = -Infinity;
  private primed = false;
  async unlock() {
    this.ctx ??= new AudioContext();
    await this.ctx.resume();
  }
  async load(clips: { path: string; sha256: string; data: string }[]) {
    if (!this.ctx) throw new Error("Sound output is not initialized");
    const decoded = await Promise.all(
      clips.map(async (c) => {
        const bytes = Uint8Array.from(atob(c.data), (x) => x.charCodeAt(0));
        return [
          c.path,
          {
            buffer: await this.ctx!.decodeAudioData(bytes.buffer),
            sha256: c.sha256,
          },
        ] as const;
      }),
    );
    for (const [path, c] of decoded) this.clips.set(path, c);
  }
  reset() {
    this.stop();
    this.cursor = -Infinity;
  }
  begin(tick: number) {
    this.stop();
    this.cursor = this.cursor === tick ? tick : tick - 0.001;
    this.primed = true;
  }
  stop() {
    for (const s of this.active) {
      s.stop();
      s.disconnect();
    }
    this.active.clear();
    this.primed = false;
  }
  advance(frame: StudioFrame, speed: number) {
    const ctx = this.ctx,
      tape = frame.context.timeline;
    if (!ctx || !tape || !this.primed) return;
    for (const e of crossedSounds(
      tape.events,
      this.cursor,
      frame.context.tick,
      tape.durationTicks,
    )) {
      const clip = this.clips.get(e.sound!);
      if (!clip) continue;
      const expected = frame.context.dependencies.find(
        (d) => d.path === e.sound,
      )?.sha256;
      if (expected && expected !== clip.sha256) {
        this.stop();
        throw new Error(
          "Звуковой ресурс изменился. Выключите и снова включите звук.",
        );
      }
      const source = ctx.createBufferSource();
      source.buffer = clip.buffer;
      source.playbackRate.value = speed;
      source.connect(ctx.destination);
      this.active.add(source);
      source.onended = () => {
        this.active.delete(source);
        source.disconnect();
      };
      source.start();
    }
    this.cursor = frame.context.tick;
  }
  setRate(rate: number) {
    for (const source of this.active) source.playbackRate.value = rate;
  }
  async dispose() {
    this.stop();
    await this.ctx?.close();
    this.ctx = undefined;
    this.clips.clear();
  }
}
