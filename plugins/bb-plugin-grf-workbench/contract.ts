import { effectLibrarySchema, libraryQuery } from "./effect-library-contract";
import { skillCatalogSchema } from "./skill-catalog-contract";
import { reviewRequestSchema, reviewEnvironmentSchema } from "./review-request";
import {
  previewSkillId,
  studioAudio,
  studioScene,
  studioScope,
  studioFrame,
  studioSnapshot,
} from "./studio-contract";
import { defineRpcContract } from "@get-bb/plugin-sdk";
import { z } from "zod";

export const id = z.string().uuid();
export const digest = z.string().regex(/^[a-f0-9]{64}$/);
export const rectSchema = z
  .object({
    x: z.number().int().nonnegative(),
    y: z.number().int().nonnegative(),
    width: z.number().int().positive(),
    height: z.number().int().positive(),
  })
  .strict();
export type Rect = z.infer<typeof rectSchema>;
export const imageSchema = z
  .object({
    digest,
    width: z.number().int().positive(),
    height: z.number().int().positive(),
    mimeType: z.enum(["image/png", "image/jpeg"]),
    bytes: z.number().int().positive(),
  })
  .strict();
export const grfEntrySchema = z
  .object({
    id: digest,
    path: z.string(),
    type: z.string(),
    archive: z.number().int(),
    source: z.string(),
    bytes: z.number().int(),
    variants: z.array(z.number().int()),
  })
  .strict();
export const grfArchiveSchema = z
  .object({
    index: z.number().int(),
    path: z.string(),
    files: z.number().int(),
    bytes: z.number().int(),
  })
  .strict();
export const grfSearchSchema = z
  .object({
    archives: z.array(grfArchiveSchema),
    fingerprint: digest,
    total: z.number().int(),
    matched: z.number().int(),
    entries: z.array(grfEntrySchema),
    next: z.number().int(),
  })
  .strict();
export const grfInfoSchema = z
  .object({
    entry: grfEntrySchema,
    version: z.string(),
    imageCount: z.number().int(),
    actions: z.array(
      z
        .object({
          index: z.number().int(),
          name: z.string(),
          frames: z.number().int(),
          intervalMs: z.number(),
        })
        .strict(),
    ),
    pair: grfEntrySchema.nullable(),
    warnings: z.array(z.string()),
  })
  .strict();
export const grfDependencySchema = z
  .object({ path: z.string(), source: z.string(), sha256: digest })
  .strict();
export const grfPreviewBase = z
  .object({
    info: grfInfoSchema,
    fingerprint: digest,
    dependencies: z.array(grfDependencySchema),
    renderer: z.string(),
    action: z.number().int(),
    firstFrame: z.number().int(),
    frameCount: z.number().int().min(1).max(256),
    width: z.number().int().positive(),
    height: z.number().int().positive(),
    columns: z.number().int().positive(),
    originX: z.number().int(),
    originY: z.number().int(),
    intervalMs: z.number().positive(),
    colorKey: z.boolean(),
  })
  .strict();
export const grfPreviewSchema = grfPreviewBase.extend({
  image: imageSchema,
  rendererVersion: digest,
});
export const grfResourceSchema = z
  .object({
    entry: grfEntrySchema,
    action: z.number().int(),
    frame: z.number().int(),
    originX: z.number().int(),
    originY: z.number().int(),
    intervalMs: z.number(),
    renderer: z.string(),
    rendererVersion: digest,
    fingerprint: digest,
    dependencies: z.array(grfDependencySchema),
    colorKey: z.boolean(),
  })
  .strict();
const grfSearchInput = z
  .object({
    query: z.string().max(200),
    type: z.string().max(12),
    archive: z.number().int().min(-1).max(15),
    offset: z.number().int().nonnegative(),
  })
  .strict();
const grfInspectInput = z
  .object({ id: digest, archive: z.number().int().min(-1).max(15) })
  .strict();
export const grfRenderInput = grfInspectInput.extend({
  action: z.number().int().min(0).max(2047),
  frame: z.number().int().min(0).max(10000),
  animated: z.boolean(),
  colorKey: z.boolean(),
});
export type GrfEntry = z.infer<typeof grfEntrySchema>;
export type GrfInfo = z.infer<typeof grfInfoSchema>;
export type GrfSearch = z.infer<typeof grfSearchSchema>;
export type GrfPreview = z.infer<typeof grfPreviewSchema> & {
  previewId: string;
};
export const captureSchema = z
  .object({
    id,
    threadId: z.string(),
    projectId: z.string(),
    hostId: z.string(),
    title: z.string(),
    sceneContext: z.string(),
    sourcePath: z.string(),
    createdAt: z.string(),
    image: imageSchema,
    resource: grfResourceSchema.optional(),
    skillFrame: studioSnapshot.optional(),
    pendingReview: id.optional(),
    copiedFrom: z
      .object({ captureId: id, threadId: z.string(), copiedAt: z.string() })
      .strict()
      .optional(),
  })
  .strict();
export const annotationSchema = z
  .object({
    id,
    captureId: id,
    imageDigest: digest,
    rect: rectSchema,
    comment: z.string(),
    createdAt: z.string(),
    resolved: z.boolean(),
    deletedAt: z.string().nullable().optional(),
    copiedFrom: z
      .object({ annotationId: id, revision: z.number().int().nonnegative() })
      .strict()
      .optional(),
    revision: z.number().int().nonnegative(),
    order: z.number().int().nonnegative().optional(),
    crop: imageSchema,
  })
  .strict();
export type Capture = z.infer<typeof captureSchema>;
export type Annotation = z.infer<typeof annotationSchema>;
export const imageDataSchema = z
  .object({
    mimeType: z.enum(["image/png", "image/jpeg"]),
    data: z.string().max(6 * 1024 * 1024),
  })
  .strict();
export const bundleSchema = z
  .object({ capture: captureSchema, annotation: annotationSchema })
  .strict();
export type Bundle = z.infer<typeof bundleSchema>;
const uploadScope = z
  .object({ threadId: z.string().min(1), uploadId: id })
  .strict();
const uploadChunk = uploadScope.extend({
  index: z.number().int().min(0).max(85),
  data: z
    .string()
    .min(1)
    .max(512 * 1024),
});
const uploadComplete = uploadScope.extend({
  fileName: z.string().min(1).max(512),
  totalBytes: z
    .number()
    .int()
    .min(24)
    .max(32 * 1024 * 1024),
  checksum: digest,
});
export const hostContract = defineRpcContract({
  effectLibrary: {
    input: libraryQuery.extend({ root: z.string() }),
    output: effectLibrarySchema,
  },
  skillCatalog: {
    input: z.object({ root: z.string() }).strict(),
    output: skillCatalogSchema,
  },
  studioAudio: {
    input: z
      .object({ root: z.string(), skillId: previewSkillId.optional() })
      .strict(),
    output: studioAudio,
  },
  studioRender: {
    input: studioScope.extend({ root: z.string(), scene: studioScene }),
    output: studioFrame,
  },
  studioClose: {
    input: studioScope,
    output: z.object({ closed: z.boolean() }).strict(),
  },
  studioCapture: {
    input: studioScope.extend({ frameId: id }),
    output: z.object({ image: imageSchema, context: studioSnapshot }).strict(),
  },
  grfSearch: {
    input: grfSearchInput.extend({ root: z.string() }),
    output: grfSearchSchema,
  },
  grfInspect: {
    input: grfInspectInput.extend({ root: z.string() }),
    output: grfInfoSchema,
  },
  grfRender: {
    input: grfRenderInput.extend({ root: z.string() }),
    output: grfPreviewSchema,
  },
  uploadChunk: {
    input: uploadChunk,
    output: z.object({ received: z.number().int() }).strict(),
  },
  finishUpload: {
    input: uploadComplete,
    output: z.object({ image: imageSchema, sourcePath: z.string() }).strict(),
  },
  discardUpload: {
    input: uploadScope,
    output: z.object({ discarded: z.boolean() }).strict(),
  },

  importImage: {
    input: z.object({ root: z.string(), path: z.string().min(1) }).strict(),
    output: z.object({ image: imageSchema, sourcePath: z.string() }).strict(),
  },
  readImage: {
    input: z
      .object({ digest, offset: z.number().int().nonnegative() })
      .strict(),
    output: imageDataSchema.extend({
      nextOffset: z.number().int(),
      done: z.boolean(),
    }),
  },
  agentImage: {
    input: z.object({ digest }).strict(),
    output: z
      .object({
        image: imageDataSchema,
        width: z.number().int(),
        height: z.number().int(),
      })
      .strict(),
  },
  cropImage: {
    input: z.object({ digest, rect: rectSchema }).strict(),
    output: imageSchema,
  },
  exportAnnotation: {
    input: z
      .object({ root: z.string(), directory: z.string(), bundle: bundleSchema })
      .strict(),
    output: z
      .object({ manifest: z.string(), original: z.string(), crop: z.string() })
      .strict(),
  },
});
export const reviewDraftSchema = z
  .object({
    id,
    title: z.string(),
    capture: captureSchema,
    annotations: z.array(annotationSchema),
    defaultEnvironment: reviewEnvironmentSchema,
    status: z.enum(["prepared", "creating", "created", "copied", "done"]),
    threadId: z.string().nullable(),
  })
  .strict();
export type ReviewDraft = z.infer<typeof reviewDraftSchema>;
export const rpcContract = defineRpcContract({
  researchContext: {
    input: z.object({ threadId: z.string() }).strict(),
    output: z
      .object({ projectId: z.string(), environmentId: z.string() })
      .strict(),
  },
  startResearch: {
    input: z
      .object({
        operationId: id,
        sourceThreadId: z.string(),
        skillId: z.number().int().min(1).max(65535),
        skillName: z.string().min(1).max(200),
        request: reviewRequestSchema,
      })
      .strict(),
    output: z.object({ threadId: z.string() }).strict(),
  },

  effectLibrary: {
    input: libraryQuery.extend({ threadId: z.string() }),
    output: effectLibrarySchema,
  },
  skillCatalog: {
    input: z.object({ threadId: z.string() }).strict(),
    output: skillCatalogSchema,
  },
  prepareReview: {
    input: z
      .object({
        operationId: id,
        captureId: id,
        imageDigest: digest,
        title: z.string().trim().min(1).max(200),
        annotations: z
          .array(
            z.object({ id, revision: z.number().int().nonnegative() }).strict(),
          )
          .max(200),
      })
      .strict(),
    output: reviewDraftSchema,
  },
  discardReviewDraft: {
    input: z.object({ operationId: id }).strict(),
    output: z.object({ discarded: z.boolean() }).strict(),
  },
  reviewDraft: {
    input: z.object({ operationId: id }).strict(),
    output: reviewDraftSchema,
  },
  startReview: {
    input: z
      .object({ operationId: id, request: reviewRequestSchema.optional() })
      .strict(),
    output: z.object({ threadId: z.string(), captureId: id }).strict(),
  },
  reviewForThread: {
    input: z.object({ threadId: z.string() }).strict(),
    output: z.object({ captureId: id, title: z.string() }).strict().nullable(),
  },

  deleteCapture: {
    input: z.object({ captureId: id, imageDigest: digest }).strict(),
    output: z
      .object({
        captureId: id,
        threadId: z.string(),
        deletedAnnotations: z.number().int().nonnegative(),
      })
      .strict(),
  },
  studioAudio: {
    input: z
      .object({ threadId: z.string(), skillId: previewSkillId.optional() })
      .strict(),
    output: studioAudio,
  },
  studioRender: {
    input: studioScope.extend({ scene: studioScene }),
    output: studioFrame,
  },
  studioClose: {
    input: studioScope,
    output: z.object({ closed: z.boolean() }).strict(),
  },
  studioCapture: {
    input: studioScope.extend({ frameId: id }),
    output: captureSchema,
  },
  grfSearch: {
    input: grfSearchInput.extend({ threadId: z.string() }),
    output: grfSearchSchema,
  },
  grfInspect: {
    input: grfInspectInput.extend({ threadId: z.string() }),
    output: grfInfoSchema,
  },
  grfRender: {
    input: grfRenderInput.extend({ threadId: z.string() }),
    output: grfPreviewSchema.extend({ previewId: id }),
  },
  grfImage: {
    input: z
      .object({ previewId: id, offset: z.number().int().nonnegative() })
      .strict(),
    output: imageDataSchema.extend({
      nextOffset: z.number().int(),
      done: z.boolean(),
    }),
  },
  grfCapture: {
    input: z
      .object({ previewId: id, frame: z.number().int().min(0).max(255) })
      .strict(),
    output: captureSchema,
  },
  uploadChunk: {
    input: uploadChunk,
    output: z.object({ received: z.number().int() }).strict(),
  },
  importUpload: {
    input: uploadComplete.extend({
      title: z.string().trim().min(1).max(200),
      sceneContext: z.string().max(4000),
    }),
    output: captureSchema,
  },
  discardUpload: {
    input: uploadScope,
    output: z.object({ discarded: z.boolean() }).strict(),
  },
  list: {
    input: z.object({ threadId: z.string() }).strict(),
    output: z.array(captureSchema).max(100),
  },
  importImage: {
    input: z
      .object({
        threadId: z.string(),
        path: z.string().min(1).max(4096),
        title: z.string().trim().min(1).max(200),
        sceneContext: z.string().max(4000),
      })
      .strict(),
    output: captureSchema,
  },
  get: {
    input: z.object({ captureId: id }).strict(),
    output: z
      .object({
        capture: captureSchema,
        annotations: z.array(annotationSchema),
      })
      .strict(),
  },
  image: {
    input: z
      .object({
        captureId: id,
        annotationId: id.optional(),
        offset: z.number().int().nonnegative(),
      })
      .strict(),
    output: imageDataSchema.extend({
      nextOffset: z.number().int(),
      done: z.boolean(),
    }),
  },
  annotate: {
    input: z
      .object({
        captureId: id,
        imageDigest: digest,
        rect: rectSchema,
        comment: z.string().trim().min(1).max(4000),
      })
      .strict(),
    output: annotationSchema,
  },
  transformAnnotation: {
    input: z
      .object({
        annotationId: id,
        revision: z.number().int().nonnegative(),
        imageDigest: digest,
        rect: rectSchema,
      })
      .strict(),
    output: annotationSchema,
  },
  updateAnnotation: {
    input: z
      .object({
        annotationId: id,
        revision: z.number().int().nonnegative(),
        comment: z.string().trim().min(1).max(4000),
        resolved: z.boolean().optional(),
      })
      .strict(),
    output: annotationSchema,
  },
  deleteAnnotation: {
    input: z
      .object({ annotationId: id, revision: z.number().int().nonnegative() })
      .strict(),
    output: annotationSchema,
  },
  restoreAnnotation: {
    input: z
      .object({ annotationId: id, revision: z.number().int().nonnegative() })
      .strict(),
    output: annotationSchema,
  },
  resolve: {
    input: z
      .object({
        annotationId: id,
        revision: z.number().int().nonnegative(),
        resolved: z.boolean(),
      })
      .strict(),
    output: annotationSchema,
  },
  annotation: {
    input: z.object({ annotationId: id }).strict(),
    output: bundleSchema,
  },
});
