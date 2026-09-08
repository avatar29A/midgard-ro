import { createHash, randomUUID } from "node:crypto";
import {
  execFile,
  spawn,
  type ChildProcessWithoutNullStreams,
} from "node:child_process";
import {
  access,
  mkdir,
  readFile,
  readdir,
  realpath,
  rename,
  rm,
} from "node:fs/promises";
import { join } from "node:path";
import { z } from "zod";
import {
  studioAudio,
  studioContext,
  studioFrame,
  type StudioFrame,
  type StudioScene,
} from "./studio-contract";
import { ImageStore } from "./image-store";

const builds = new Map<string, Promise<void>>();
async function sourceFingerprint(root: string) {
  const hash = createHash("sha256").update(
    `${process.platform}/${process.arch}`,
  );
  async function walk(dir: string): Promise<void> {
    for (const e of (
      await readdir(join(root, dir), { withFileTypes: true })
    ).sort((a, b) => a.name.localeCompare(b.name))) {
      const p = join(dir, e.name);
      if (e.isDirectory() && !["testdata", "node_modules"].includes(e.name))
        await walk(p);
      else if (
        e.isFile() &&
        !p.endsWith("_test.go") &&
        /\.(go|vert|frag|glsl|yaml|json|ttf|otf|txt)$/.test(p)
      ) {
        hash.update(p);
        hash.update(await readFile(join(root, p)));
      }
    }
  }
  for (const p of ["go.mod", "go.sum"]) {
    hash.update(p);
    hash.update(await readFile(join(root, p)));
  }
  for (const p of ["cmd/skillstudio", "internal", "pkg"]) await walk(p);
  return hash.digest("hex");
}
async function binary(root: string, dataDir: string, signal: AbortSignal) {
  const version = await sourceFingerprint(root),
    dir = join(dataDir, "skill-engine"),
    path = join(dir, version);
  try {
    await access(path);
    return { path, version };
  } catch {}
  let build = builds.get(path);
  if (!build) {
    build = (async () => {
      await mkdir(dir, { recursive: true });
      const temp = `${path}.${randomUUID()}.tmp`;
      try {
        await new Promise<void>((resolve, reject) => {
          execFile(
            "go",
            [
              "build",
              "-trimpath",
              "-buildvcs=false",
              "-o",
              temp,
              "./cmd/skillstudio",
            ],
            { cwd: root, signal, timeout: 120000, maxBuffer: 1024 * 1024 },
            (err, _out, stderr) =>
              err
                ? reject(new Error(stderr.trim().slice(0, 3000) || err.message))
                : resolve(),
          );
        });
        if ((await sourceFingerprint(root)) !== version)
          throw new Error(
            "Исходники изменились во время сборки. Нажмите «Пересобрать» ещё раз.",
          );
        await rename(temp, path);
      } finally {
        await rm(temp, { force: true });
      }
    })();
    builds.set(path, build);
  }
  try {
    await build;
  } finally {
    if (builds.get(path) === build) builds.delete(path);
  }
  return { path, version };
}

// One request in flight per session. No frame queue, no permanent live-frame
// files. Only explicit captures enter ImageStore. Session ownership is checked
// on every call, and idle sessions/lifecycle disposal release native resources.
class Session {
  frames = new Map<string, StudioFrame>();
  lastUsed = Date.now();
  child?: ChildProcessWithoutNullStreams;
  busy = false;
  disposed = false;
  error = "";
  private pending?: {
    resolve: (v: unknown) => void;
    reject: (e: Error) => void;
  };
  constructor(
    readonly root: string,
    readonly threadId: string,
    readonly version: string,
    private release: () => Promise<void>,
  ) {}
  start(path: string) {
    if (this.disposed) throw new Error("Studio session closed");
    const child = spawn(path, [], {
      cwd: this.root,
      stdio: ["pipe", "pipe", "pipe"],
    });
    this.child = child;
    let buffer = "";
    child.stdout.setEncoding("utf8");
    child.stderr.setEncoding("utf8");
    child.stderr.on("data", (s: string) => {
      this.error = (this.error + s).slice(-3000);
    });
    child.stdout.on("data", (s: string) => {
      buffer += s;
      if (buffer.length > 7 * 1024 * 1024) {
        this.fail("Studio frame exceeded output limit");
        return;
      }
      let i: number;
      while ((i = buffer.indexOf("\n")) >= 0) {
        const line = buffer.slice(0, i);
        buffer = buffer.slice(i + 1);
        const pending = this.pending;
        this.pending = undefined;
        if (!pending) {
          this.fail("Unexpected renderer output");
          return;
        }
        try {
          pending.resolve(JSON.parse(line));
        } catch {
          pending.reject(new Error("Invalid renderer JSON"));
          this.close();
        }
      }
    });
    child.on("error", (e) => this.fail(e.message));
    child.on("exit", () =>
      this.fail(this.error || "Renderer exited. Rebuild and reopen the scene."),
    );
    child.stdin.on("error", (e) => this.fail(e.message));
  }
  fail(message: string) {
    this.error = message;
    this.pending?.reject(new Error(message));
    this.pending = undefined;
    this.close();
  }
  close() {
    if (this.disposed) return;
    this.disposed = true;
    this.pending?.reject(new Error(this.error || "Studio session closed"));
    this.pending = undefined;
    this.child?.stdin.end();
    this.child?.kill();
    this.frames.clear();
    void this.release();
  }
  async render(scene: StudioScene, signal: AbortSignal) {
    if (this.busy) throw new Error("Studio renderer is busy");
    if (this.disposed) throw new Error(this.error || "Studio session closed");
    signal.throwIfAborted();
    this.busy = true;
    this.lastUsed = Date.now();
    const abort = () => this.fail("Studio render cancelled");
    signal.addEventListener("abort", abort, { once: true });
    const timeout = setTimeout(
      () => this.fail("Studio frame timed out"),
      25000,
    );
    try {
      const raw = await new Promise<unknown>((resolve, reject) => {
        this.pending = { resolve, reject };
        this.child!.stdin.write(JSON.stringify(scene) + "\n");
      });
      if (raw && typeof raw === "object" && "error" in raw)
        throw new Error(String(raw.error));
      const result = z
        .object({
          png: z.string().max(6 * 1024 * 1024),
          context: studioContext,
        })
        .strict()
        .parse(raw);
      const frame = studioFrame.parse({
        ...result,
        frameId: randomUUID(),
        context: { ...result.context, rendererVersion: this.version },
      });
      this.frames.set(frame.frameId, frame);
      while (this.frames.size > 12)
        this.frames.delete(this.frames.keys().next().value!);
      return frame;
    } finally {
      clearTimeout(timeout);
      signal.removeEventListener("abort", abort);
      this.busy = false;
    }
  }
}

type Scope = { sessionId: string; threadId: string };
type HostContext = {
  signal: AbortSignal;
  lifecycle: { signal: AbortSignal };
  experimental_paths: { dataDir: string };
  experimental_retainWorker: () => { dispose: () => Promise<void> };
};
export class StudioBridge {
  private sessions = new Map<string, Promise<Session>>();
  async render(
    input: Scope & { root: string; scene: StudioScene },
    ctx: HostContext,
  ) {
    let promise = this.sessions.get(input.sessionId);
    if (!promise) {
      if (this.sessions.size >= 4)
        throw new Error("Закройте одну из четырёх сцен Skill Studio.");
      promise = this.create(input, ctx);
      this.sessions.set(input.sessionId, promise);
    }
    let session: Session;
    try {
      session = await promise;
    } catch (e) {
      if (this.sessions.get(input.sessionId) === promise)
        this.sessions.delete(input.sessionId);
      throw e;
    }
    if (
      session.threadId !== input.threadId ||
      session.root !== (await realpath(input.root))
    )
      throw new Error("Studio session belongs to another workspace/thread");
    if (session.disposed) {
      this.sessions.delete(input.sessionId);
      throw new Error(
        session.error || "Сессия завершена. Нажмите «Пересобрать». ",
      );
    }
    return session.render(input.scene, ctx.signal);
  }
  private async create(input: Scope & { root: string }, ctx: HostContext) {
    const lease = ctx.experimental_retainWorker();
    let session: Session | undefined;
    const abort = () => session?.close();
    ctx.lifecycle.signal.addEventListener("abort", abort, { once: true });
    try {
      const root = await realpath(input.root);
      const engine = await binary(
        root,
        ctx.experimental_paths.dataDir,
        ctx.signal,
      );
      ctx.signal.throwIfAborted();
      ctx.lifecycle.signal.throwIfAborted();
      let timer: ReturnType<typeof setInterval>;
      session = new Session(root, input.threadId, engine.version, async () => {
        clearInterval(timer);
        ctx.lifecycle.signal.removeEventListener("abort", abort);
        await lease.dispose();
      });
      const current = session;
      timer = setInterval(() => {
        if (!current.busy && Date.now() - current.lastUsed > 120000) {
          current.close();
          this.sessions.delete(input.sessionId);
        }
      }, 30000);
      timer.unref();
      session.start(engine.path);
      return session;
    } catch (e) {
      ctx.lifecycle.signal.removeEventListener("abort", abort);
      await lease.dispose();
      throw e;
    }
  }
  async close(input: Scope) {
    const p = this.sessions.get(input.sessionId);
    if (!p) return { closed: false };
    const s = await p;
    if (s.threadId !== input.threadId) throw new Error("Wrong session owner");
    s.close();
    if (this.sessions.get(input.sessionId) === p)
      this.sessions.delete(input.sessionId);
    return { closed: true };
  }
  async capture(input: Scope & { frameId: string }, dataDir: string) {
    const s = await this.sessions.get(input.sessionId);
    if (!s || s.threadId !== input.threadId)
      throw new Error("Сессия не найдена");
    const f = s.frames.get(input.frameId);
    if (!f)
      throw new Error(
        "Кадр уже недоступен. Остановите просмотр и сохраните текущий кадр.",
      );
    s.lastUsed = Date.now();
    const image = await new ImageStore(dataDir).importBytes(
      Buffer.from(f.png, "base64"),
    );
    return { image, context: f.context };
  }
  async dispose() {
    await Promise.allSettled(
      [...this.sessions.values()].map(async (p) => (await p).close()),
    );
    this.sessions.clear();
  }
}

export async function studioAudioRequest(
  root: string,
  dataDir: string,
  signal: AbortSignal,
  skillId = 13,
) {
  const engine = await binary(root, dataDir, signal);
  const result = await new Promise<string>((resolve, reject) => {
    execFile(
      engine.path,
      ["--audio", String(skillId)],
      { cwd: root, signal, timeout: 25000, maxBuffer: 5 * 1024 * 1024 },
      (err, stdout, stderr) =>
        err ? reject(new Error(stderr.trim() || err.message)) : resolve(stdout),
    );
  });
  return studioAudio.parse(JSON.parse(result));
}
