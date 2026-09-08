import { effectLibrarySchema } from "./effect-library-contract";
import { skillCatalogSchema } from "./skill-catalog-contract";
import { StudioBridge, studioAudioRequest } from "./studio-bridge";
const studio = new StudioBridge();
import { experimental_defineHostEntry } from "@get-bb/plugin-sdk/host";
import { grfRequest } from "./grf-bridge";
import { grfSearchSchema, grfInfoSchema, grfPreviewSchema } from "./contract";
import { hostContract } from "./contract";
import { ImageStore } from "./image-store";
import { Uploads } from "./uploads";
export default experimental_defineHostEntry({
  contract: hostContract,
  dispose: () => studio.dispose(),
  handlers: {
    effectLibrary: async ({ root, kind, ...query }, ctx) =>
      effectLibrarySchema.parse(
        await grfRequest(root, ctx.experimental_paths.dataDir, ctx.signal, {
          ...query,
          type: kind,
          op: "library",
          limit: 50,
        }),
      ),
    skillCatalog: async ({ root }, ctx) =>
      skillCatalogSchema.parse(
        await grfRequest(root, ctx.experimental_paths.dataDir, ctx.signal, {
          op: "skills",
        }),
      ),
    studioAudio: (input, ctx) =>
      studioAudioRequest(
        input.root,
        ctx.experimental_paths.dataDir,
        ctx.signal,
        input.skillId,
      ),
    studioRender: (input, ctx) => studio.render(input, ctx),
    studioClose: (input) => studio.close(input),
    studioCapture: (input, ctx) =>
      studio.capture(input, ctx.experimental_paths.dataDir),
    grfSearch: async ({ root, ...input }, ctx) =>
      grfSearchSchema.parse(
        await grfRequest(root, ctx.experimental_paths.dataDir, ctx.signal, {
          op: "search",
          ...input,
          limit: 50,
        }),
      ),
    grfInspect: async ({ root, ...input }, ctx) =>
      grfInfoSchema.parse(
        await grfRequest(root, ctx.experimental_paths.dataDir, ctx.signal, {
          op: "inspect",
          ...input,
        }),
      ),
    grfRender: async ({ root, ...input }, ctx) =>
      grfPreviewSchema.parse(
        await grfRequest(root, ctx.experimental_paths.dataDir, ctx.signal, {
          op: "render",
          ...input,
        }),
      ),
    uploadChunk: (input, ctx) =>
      new Uploads(
        ctx.experimental_paths.tempDir,
        new ImageStore(ctx.experimental_paths.dataDir),
      ).chunk(input.threadId, input.uploadId, input.index, input.data),
    finishUpload: (input, ctx) =>
      new Uploads(
        ctx.experimental_paths.tempDir,
        new ImageStore(ctx.experimental_paths.dataDir),
      ).finish(
        input.threadId,
        input.uploadId,
        input.fileName,
        input.totalBytes,
        input.checksum,
      ),
    discardUpload: (input, ctx) =>
      new Uploads(
        ctx.experimental_paths.tempDir,
        new ImageStore(ctx.experimental_paths.dataDir),
      ).discard(input.threadId, input.uploadId),
    importImage: (input, ctx) =>
      new ImageStore(ctx.experimental_paths.dataDir).importImage(
        input.root,
        input.path,
      ),
    readImage: (input, ctx) =>
      new ImageStore(ctx.experimental_paths.dataDir).image(
        input.digest,
        input.offset,
      ),
    agentImage: (input, ctx) =>
      new ImageStore(ctx.experimental_paths.dataDir).agentImage(input.digest),
    cropImage: (input, ctx) =>
      new ImageStore(ctx.experimental_paths.dataDir).cropImage(
        input.digest,
        input.rect,
      ),
    exportAnnotation: (input, ctx) =>
      new ImageStore(ctx.experimental_paths.dataDir).exportAnnotation(
        input.root,
        input.directory,
        input.bundle,
      ),
  },
});
