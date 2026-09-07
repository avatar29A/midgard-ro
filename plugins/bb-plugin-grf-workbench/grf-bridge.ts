import { createHash } from "node:crypto";
import { execFile } from "node:child_process";
import { access, mkdir, readFile, readdir, realpath } from "node:fs/promises";
import { join } from "node:path";
import { grfInfoSchema, grfPreviewBase, grfSearchSchema } from "./contract";
import { ImageStore } from "./image-store";
import { z } from "zod";

const builds = new Map<string, Promise<void>>();
function run(
  command: string,
  args: string[],
  root: string,
  signal: AbortSignal,
  input?: string,
): Promise<string> {
  return new Promise((resolve, reject) => {
    const child = execFile(
      command,
      args,
      {
        cwd: root,
        signal,
        timeout: 25000,
        maxBuffer: 7 * 1024 * 1024,
        encoding: "utf8",
        env: { ...process.env, GOMEMLIMIT: "256MiB", GOMAXPROCS: "2" },
      },
      (err, stdout, stderr) => {
        if (err) {
          let message = stderr.trim() || err.message;
          try {
            message = JSON.parse(stdout).error || message;
          } catch {}
          reject(new Error(message.slice(0, 2000)));
          return;
        }
        resolve(stdout);
      },
    );
    child.stdin?.on("error", () => {});
    child.stdin?.end(input);
  });
}
async function binary(root: string, dataDir: string, signal: AbortSignal) {
  root = await realpath(root);
  const inputs = ["go.mod", "go.sum"];
  async function walk(dir: string) {
    for (const e of (
      await readdir(join(root, dir), { withFileTypes: true })
    ).sort((a, b) => a.name.localeCompare(b.name))) {
      const p = `${dir}/${e.name}`;
      if (e.isDirectory() && e.name !== "testdata") await walk(p);
      else if (e.isFile() && p.endsWith(".go") && !p.endsWith("_test.go"))
        inputs.push(p);
    }
  }
  for (const p of [
    "cmd/grfworkbench",
    "internal/assetworkbench",
    "pkg/grf",
    "pkg/formats",
    "pkg/encoding",
    "internal/engine/charsprite",
    "internal/engine/sprite",
    "internal/engine/texture",
  ])
    await walk(p);
  const hash = createHash("sha256");
  hash.update(`${process.platform}/${process.arch}`);
  for (const p of inputs) {
    hash.update(p);
    hash.update(await readFile(join(root, p)));
  }
  const version = hash.digest("hex"),
    path = join(dataDir, "grf-engine", version);
  try {
    await access(path);
    return { path, version };
  } catch {}
  let building = builds.get(path);
  if (!building) {
    building = (async () => {
      await mkdir(join(dataDir, "grf-engine"), { recursive: true });
      await run(
        "go",
        [
          "build",
          "-trimpath",
          "-buildvcs=false",
          "-o",
          path,
          "./cmd/grfworkbench",
        ],
        root,
        signal,
      );
    })();
    builds.set(path, building);
  }
  try {
    await building;
  } finally {
    if (builds.get(path) === building) builds.delete(path);
  }
  return { path, version };
}
export async function grfRequest(
  root: string,
  dataDir: string,
  signal: AbortSignal,
  request: Record<string, unknown>,
) {
  const engine = await binary(root, dataDir, signal);
  const data = JSON.parse(
    await run(engine.path, [], root, signal, JSON.stringify(request)),
  );
  if (request.op === "search") return grfSearchSchema.parse(data);
  if (request.op === "inspect") return grfInfoSchema.parse(data);
  const parsed = grfPreviewBase
    .extend({ png: z.string().max(6 * 1024 * 1024) })
    .parse(data);
  const { png, ...preview } = parsed;
  const image = await new ImageStore(dataDir).importBytes(
    Buffer.from(png, "base64"),
  );
  return { ...preview, image, rendererVersion: engine.version };
}
