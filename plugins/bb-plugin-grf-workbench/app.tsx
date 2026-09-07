import { SkillStudio } from "./skill-studio";
import type { StudioScene } from "./studio-contract";
import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react";
import type { PointerEvent, CSSProperties } from "react";
import {
  definePluginApp,
  useBbNavigate,
  useComposer,
  useRealtime,
  useRealtimeConnectionState,
  useRpc,
} from "@get-bb/plugin-sdk/app";
import type { rpcContract, Annotation, Capture, Rect } from "./contract";
import {
  imagePoint,
  selectionRect,
  resizeRect,
  moveRect,
  resizeHandles,
  type ResizeHandle,
  type Point,
} from "./geometry";
import { Button } from "./components/ui/button";
import { Input } from "./components/ui/input";
import "./app.css";
import { AssetBrowser } from "./asset-browser";
import { uploadPickedImage } from "./file-upload";

const uuid = (v: unknown): string | undefined =>
  typeof v === "string" && /^[a-f0-9-]{36}$/.test(v) ? v : undefined;
const errorText = (e: unknown) => (e instanceof Error ? e.message : String(e));
async function loadImage(
  rpc: ReturnType<typeof useRpc<typeof rpcContract>>,
  captureId: string,
  annotationId?: string,
  isLive: () => boolean = () => true,
) {
  const chunks: string[] = [];
  let offset = 0;
  while (isLive()) {
    const chunk = await rpc.call("image", {
      captureId,
      offset,
      ...(annotationId ? { annotationId } : {}),
    });
    chunks.push(chunk.data);
    if (chunk.done) return `data:${chunk.mimeType};base64,${chunks.join("")}`;
    offset = chunk.nextOffset;
  }
  return "";
}
function Review({
  threadId,
  captureId,
  annotationId,
  onThread,
  onStudio,
}: {
  threadId: string | null;
  captureId?: string;
  annotationId?: string;
  onThread?: (threadId: string) => void;
  onStudio?: (scene: StudioScene) => void;
}) {
  const rpc = useRpc<typeof rpcContract>(),
    composer = useComposer(),
    navigate = useBbNavigate();
  const [captures, setCaptures] = useState<Capture[]>([]);
  const [selected, setSelected] = useState(captureId ?? "");
  const [capture, setCapture] = useState<Capture | null>(null);
  const [annotations, setAnnotations] = useState<Annotation[]>([]);
  const [imageURL, setImageURL] = useState("");
  const [cropURL, setCropURL] = useState("");
  const [activeAnnotation, setActiveAnnotation] = useState(annotationId ?? "");
  const [annotationComment, setAnnotationComment] = useState("");
  const [annotationRevision, setAnnotationRevision] = useState(0);
  const [deletedAnnotation, setDeletedAnnotation] = useState<Annotation | null>(
    null,
  );
  const selectedCard = useRef<HTMLElement | null>(null);
  const suppressRegionClick = useRef(false);
  const [selection, setSelection] = useState<Rect | null>(null);
  const [transform, setTransform] = useState<{
    mode: "resize" | "move";
    annotationId: string | null;
    revision: number;
    imageDigest: string;
    original: Rect;
    rect: Rect;
  } | null>(null);
  const transformDrag = useRef<{
    handle: ResizeHandle | "move";
    start: Point;
    rect: Rect;
    pointerId: number;
  } | null>(null);
  const [transformCursor, setTransformCursor] = useState<string | undefined>();
  const [comment, setComment] = useState("");
  const [path, setPath] = useState("");
  const [title, setTitle] = useState("");
  const [context, setContext] = useState("");
  const [importOpen, setImportOpen] = useState(false);
  const fileInput = useRef<HTMLInputElement>(null);
  const uploadController = useRef<AbortController | null>(null);
  useEffect(() => () => uploadController.current?.abort(), []);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [pending, setPending] = useState(false);
  const [zoom, setZoom] = useState<number | null>(null);
  const [viewportWidth, setViewportWidth] = useState(700);
  const [mode, setMode] = useState<"select" | "pan">("select");
  const viewport = useRef<HTMLDivElement>(null),
    imageRef = useRef<HTMLImageElement>(null);
  const [spaceHeld, setSpaceHeld] = useState(false);
  const [panning, setPanning] = useState(false);
  const space = useRef(false);
  const hovered = useRef(false);
  const zoomAnchor = useRef<{
    x: number;
    y: number;
    clientX: number;
    clientY: number;
  } | null>(null);
  const drag = useRef<{
    mode: "select" | "pan";
    start: Point;
    clientX: number;
    clientY: number;
    scrollX: number;
    scrollY: number;
  } | null>(null);
  const activeThread = threadId ?? capture?.threadId ?? null;
  useEffect(() => {
    if (activeThread) onThread?.(activeThread);
  }, [activeThread, onThread]);
  const report = useCallback((e: unknown) => setError(errorText(e)), []);
  const [generation, setGeneration] = useState(0);
  const refresh = useCallback(() => setGeneration((n) => n + 1), []);
  useRealtime("review-changed", refresh);
  const connection = useRealtimeConnectionState();
  useEffect(() => {
    if (connection === "connected") refresh();
  }, [connection, refresh]);
  useEffect(() => {
    if (!activeThread) return;
    let live = true;
    rpc.call("list", { threadId: activeThread }).then(
      (rows) => {
        if (live) {
          setCaptures(rows);
          setSelected((current) => current || rows[0]?.id || "");
        }
      },
      (e) => {
        if (live) report(e);
      },
    );
    return () => {
      live = false;
    };
  }, [activeThread, rpc, generation, report]);
  useEffect(() => {
    if (captureId) setSelected(captureId);
    if (!annotationId) return;
    let live = true;
    rpc.call("annotation", { annotationId }).then(
      (b) => {
        if (live) {
          setSelected(b.capture.id);
          setActiveAnnotation(b.annotation.id);
        }
      },
      (e) => {
        if (live) report(e);
      },
    );
    return () => {
      live = false;
    };
  }, [captureId, annotationId, rpc, report]);
  useEffect(() => {
    setSelection(null);
    setTransform(null);
    transformDrag.current = null;
    setTransformCursor(undefined);
    setComment("");
    setImageURL("");
    setCapture(null);
    setAnnotations([]);
    setZoom(null);
    zoomAnchor.current = null;
    setPanning(false);
    setError("");
    drag.current = null;
    if (!selected) return;
    let live = true;
    Promise.all([
      rpc.call("get", { captureId: selected }),
      loadImage(rpc, selected, undefined, () => live),
    ]).then(
      ([result, img]) => {
        if (!live) return;
        setCapture(result.capture);
        setAnnotations(result.annotations);
        setImageURL(img);
      },
      (e) => {
        if (live) report(e);
      },
    );
    return () => {
      live = false;
    };
  }, [selected, rpc, report]);
  useEffect(() => {
    if (!selected) return;
    let live = true;
    rpc.call("get", { captureId: selected }).then(
      (result) => {
        if (live) setAnnotations(result.annotations);
      },
      (e) => {
        if (live) report(e);
      },
    );
    return () => {
      live = false;
    };
  }, [selected, generation, rpc, report]);
  const active = annotations.find((a) => a.id === activeAnnotation);
  useEffect(() => {
    if (active) {
      setAnnotationComment(active.comment);
      setAnnotationRevision(active.revision);
    }
    selectedCard.current?.scrollIntoView?.({ block: "nearest" });
    setTransform(null);
    transformDrag.current = null;
    setTransformCursor(undefined);
  }, [active?.id]);
  useEffect(() => {
    setCropURL("");
    if (!active) return;
    let live = true;
    loadImage(rpc, active.captureId, active.id, () => live).then(
      (img) => {
        if (live) setCropURL(img);
      },
      (e) => {
        if (live) report(e);
      },
    );
    return () => {
      live = false;
    };
  }, [active?.id, active?.crop.digest, rpc, report]);
  useEffect(() => {
    if (!viewport.current) return;
    const obs = new ResizeObserver((entries) => {
      if (entries[0]) setViewportWidth(entries[0].contentRect.width);
    });
    obs.observe(viewport.current);
    return () => obs.disconnect();
  }, [Boolean(capture)]);
  const scale =
    zoom ??
    Math.min(
      1,
      Math.max(0.05, (viewportWidth - 32) / (capture?.image.width ?? 1)),
    );
  // Use a non-passive listener so wheel/pinch zoom does not also scroll
  // the page or zoom BB itself. Only this viewport owns the gesture.
  useEffect(() => {
    const view = viewport.current;
    if (!view || !capture) return;
    const wheel = (e: WheelEvent) => {
      e.preventDefault();
      if (
        drag.current ||
        transformDrag.current ||
        !imageRef.current ||
        e.deltaY === 0
      )
        return;
      const bounds = imageRef.current.getBoundingClientRect();
      if (!bounds.width || !bounds.height) return;
      const unit =
        e.deltaMode === 1 ? 16 : e.deltaMode === 2 ? view.clientHeight : 1;
      const delta = Math.max(-500, Math.min(500, e.deltaY * unit));
      const next = Math.max(
        0.05,
        Math.min(8, scale * Math.exp(-delta * 0.002)),
      );
      if (next === scale) return;
      zoomAnchor.current = {
        x: (e.clientX - bounds.left) / scale,
        y: (e.clientY - bounds.top) / scale,
        clientX: e.clientX,
        clientY: e.clientY,
      };
      setZoom(next);
    };
    view.addEventListener("wheel", wheel, { passive: false });
    return () => view.removeEventListener("wheel", wheel);
  }, [capture, scale]);
  useLayoutEffect(() => {
    const anchor = zoomAnchor.current,
      view = viewport.current,
      img = imageRef.current;
    if (!anchor || !view || !img) return;
    zoomAnchor.current = null;
    const bounds = img.getBoundingClientRect();
    view.scrollLeft += bounds.left + anchor.x * scale - anchor.clientX;
    view.scrollTop += bounds.top + anchor.y * scale - anchor.clientY;
  }, [scale]);
  useEffect(() => {
    const release = () => {
      space.current = false;
      setSpaceHeld(false);
    };
    const keydown = (e: KeyboardEvent) => {
      if (e.code !== "Space" || e.altKey || e.ctrlKey || e.metaKey) return;
      if (
        e.target instanceof Element &&
        e.target.closest(
          "input, textarea, select, button, [contenteditable]:not([contenteditable='false']), [role='textbox']",
        )
      )
        return;
      if (
        !hovered.current &&
        !viewport.current?.contains(document.activeElement)
      )
        return;
      e.preventDefault();
      space.current = true;
      setSpaceHeld(true);
    };
    const keyup = (e: KeyboardEvent) => {
      if (e.code === "Space") release();
    };
    const blur = () => {
      release();
      drag.current = null;
      transformDrag.current = null;
      setTransformCursor(undefined);
      setPanning(false);
    };
    window.addEventListener("keydown", keydown);
    window.addEventListener("keyup", keyup);
    window.addEventListener("blur", blur);
    return () => {
      window.removeEventListener("keydown", keydown);
      window.removeEventListener("keyup", keyup);
      window.removeEventListener("blur", blur);
    };
  }, []);
  const point = (e: PointerEvent) =>
    capture && imageRef.current
      ? imagePoint(
          e.clientX,
          e.clientY,
          imageRef.current.getBoundingClientRect(),
          capture.image.width,
          capture.image.height,
        )
      : null;
  const down = (e: PointerEvent<HTMLDivElement>) => {
    if (
      pending ||
      drag.current ||
      transformDrag.current ||
      (e.button !== 0 && e.button !== 1) ||
      !capture
    )
      return;
    const gestureMode =
      e.button === 1 || space.current || mode === "pan" ? "pan" : "select";
    suppressRegionClick.current = gestureMode === "pan";
    if (transform && gestureMode === "select") {
      const target = e.target instanceof Element ? e.target : null;
      const handle = target
        ?.closest("[data-resize-handle]")
        ?.getAttribute("data-resize-handle") as ResizeHandle | undefined;
      const moving =
        transform.mode === "move" && target?.closest("[data-transform-body]");
      if ((handle && resizeHandles.includes(handle)) || moving) {
        const start = point(e);
        if (!start) return;
        e.preventDefault();
        e.stopPropagation();
        viewport.current?.focus({ preventScroll: true });
        e.currentTarget.setPointerCapture(e.pointerId);
        transformDrag.current = {
          handle: moving ? "move" : handle!,
          start,
          rect: transform.rect,
          pointerId: e.pointerId,
        };
        setTransformCursor(moving ? "move" : handleCursor(handle!));
        suppressRegionClick.current = true;
      }
      return;
    }
    if (
      gestureMode === "select" &&
      e.target instanceof Element &&
      e.target.closest("[data-annotation-id]")
    )
      return;
    if (
      gestureMode === "select" &&
      !imageRef.current?.parentElement?.contains(e.target as Node)
    )
      return;
    const p = point(e);
    if (!p) return;
    e.preventDefault();
    viewport.current?.focus({ preventScroll: true });
    e.currentTarget.setPointerCapture(e.pointerId);
    drag.current = {
      mode: gestureMode,
      start: p,
      clientX: e.clientX,
      clientY: e.clientY,
      scrollX: viewport.current?.scrollLeft ?? 0,
      scrollY: viewport.current?.scrollTop ?? 0,
    };
    setPanning(gestureMode === "pan");
    if (gestureMode === "select") {
      setSelection(null);
      setActiveAnnotation("");
    }
  };
  const move = (e: PointerEvent<HTMLDivElement>) => {
    const edit = transformDrag.current;
    if (edit && capture) {
      if (edit.pointerId !== e.pointerId) return;
      const p = point(e);
      if (!p) return;
      const delta = { x: p.x - edit.start.x, y: p.y - edit.start.y };
      const rect =
        edit.handle === "move"
          ? moveRect(
              edit.rect,
              delta,
              capture.image.width,
              capture.image.height,
            )
          : resizeRect(
              edit.rect,
              edit.handle,
              delta,
              capture.image.width,
              capture.image.height,
            );
      setTransform((old) => (old ? { ...old, rect } : old));
      return;
    }
    const d = drag.current;
    if (!d) return;
    if (d.mode === "pan" && viewport.current) {
      viewport.current.scrollLeft = d.scrollX - (e.clientX - d.clientX);
      viewport.current.scrollTop = d.scrollY - (e.clientY - d.clientY);
    } else {
      const p = point(e);
      if (p) setSelection(selectionRect(d.start, p));
    }
  };
  const up = (e: PointerEvent<HTMLDivElement>) => {
    move(e);
    transformDrag.current = null;
    setTransformCursor(undefined);
    drag.current = null;
    setPanning(false);
    if (e.currentTarget.hasPointerCapture(e.pointerId))
      e.currentTarget.releasePointerCapture(e.pointerId);
  };
  async function run(action: () => Promise<void>) {
    setPending(true);
    setError("");
    setNotice("");
    try {
      await action();
    } catch (e) {
      report(e);
    } finally {
      setPending(false);
    }
  }
  async function save() {
    if (!capture || !selection || !comment.trim()) return;
    await run(async () => {
      const a = await rpc.call("annotate", {
        captureId: capture.id,
        imageDigest: capture.image.digest,
        rect: selection,
        comment,
      });
      setAnnotations((old) => [...old.filter((x) => x.id !== a.id), a]);
      setActiveAnnotation(a.id);
      setSelection(null);
      setComment("");
      setNotice(
        "Замечание сохранено. Добавьте его в обсуждение, чтобы агент увидел область.",
      );
      refresh();
    });
  }
  function addToChat(a: Annotation) {
    composer.insertMention({
      provider: "region",
      id: a.id,
      label: `Область: ${a.comment.slice(0, 55)}`,
    });
    composer.focus();
    setNotice("Замечание добавлено в черновик сообщения.");
  }
  function chooseAnnotation(a: Annotation) {
    if (pending) return;
    cancelTransform();
    setActiveAnnotation(a.id);
    setSelection(null);
    setAnnotationComment(a.comment);
    setAnnotationRevision(a.revision);
    viewport.current?.focus({ preventScroll: true });
  }
  function applyAnnotation(a: Annotation) {
    setAnnotations((old) => old.map((x) => (x.id === a.id ? a : x)));
    if (a.id === activeAnnotation) {
      setAnnotationComment(a.comment);
      setAnnotationRevision(a.revision);
    }
    refresh();
  }
  async function saveAnnotation(a: Annotation, resolved?: boolean) {
    await run(async () => {
      const saved = await rpc.call("updateAnnotation", {
        annotationId: a.id,
        revision: annotationRevision,
        comment: annotationComment,
        ...(resolved === undefined ? {} : { resolved }),
      });
      applyAnnotation(saved);
    });
  }
  async function removeAnnotation(a: Annotation) {
    if (pending) return;
    await run(async () => {
      const removed = await rpc.call("deleteAnnotation", {
        annotationId: a.id,
        revision: a.id === activeAnnotation ? annotationRevision : a.revision,
      });
      setAnnotations((old) => old.filter((x) => x.id !== a.id));
      if (a.id === activeAnnotation) {
        setActiveAnnotation("");
        setTransform(null);
      }
      setDeletedAnnotation(removed);
      setNotice("Область удалена.");
      refresh();
      viewport.current?.focus({ preventScroll: true });
    });
  }
  async function restoreAnnotation() {
    if (!deletedAnnotation) return;
    const deleted = deletedAnnotation;
    await run(async () => {
      const restored = await rpc.call("restoreAnnotation", {
        annotationId: deleted.id,
        revision: deleted.revision,
      });
      setAnnotations((old) =>
        [...old.filter((a) => a.id !== restored.id), restored].sort(
          (a, b) =>
            a.createdAt.localeCompare(b.createdAt) || a.id.localeCompare(b.id),
        ),
      );
      setDeletedAnnotation(null);
      setActiveAnnotation(restored.id);
      setAnnotationComment(restored.comment);
      setAnnotationRevision(restored.revision);
      setNotice("Область восстановлена.");
      refresh();
    });
  }
  function handleCursor(handle: ResizeHandle) {
    return handle === "n" || handle === "s"
      ? "ns-resize"
      : handle === "e" || handle === "w"
        ? "ew-resize"
        : handle === "nw" || handle === "se"
          ? "nwse-resize"
          : "nesw-resize";
  }
  function cancelTransform() {
    const pointer = transformDrag.current;
    transformDrag.current = null;
    setTransformCursor(undefined);
    setTransform(null);
    if (pointer && viewport.current?.hasPointerCapture(pointer.pointerId))
      viewport.current.releasePointerCapture(pointer.pointerId);
  }
  function startTransform(nextMode: "resize" | "move") {
    if (pending || drag.current || transformDrag.current) return;
    if (transform) {
      if (transform.mode === nextMode) void finishTransform();
      else setTransform({ ...transform, mode: nextMode });
      return;
    }
    const rect = selection ?? active?.rect;
    if (!rect || !capture) return;
    setMode("select");
    setTransform({
      mode: nextMode,
      annotationId: selection ? null : active!.id,
      revision: annotationRevision,
      imageDigest: capture.image.digest,
      original: { ...rect },
      rect: { ...rect },
    });
    viewport.current?.focus({ preventScroll: true });
  }
  async function finishTransform() {
    if (!transform || pending || transformDrag.current) return;
    const edit = transform;
    if (!edit.annotationId) {
      setSelection(edit.rect);
      setTransform(null);
      return;
    }
    if (
      edit.rect.x === edit.original.x &&
      edit.rect.y === edit.original.y &&
      edit.rect.width === edit.original.width &&
      edit.rect.height === edit.original.height
    ) {
      setTransform(null);
      return;
    }
    await run(async () => {
      const saved = await rpc.call("transformAnnotation", {
        annotationId: edit.annotationId!,
        revision: edit.revision,
        imageDigest: edit.imageDigest,
        rect: edit.rect,
      });
      setAnnotations((old) => old.map((a) => (a.id === saved.id ? saved : a)));
      setAnnotationRevision(saved.revision);
      setTransform(null);
      refresh();
      setNotice("Размер и положение области сохранены.");
    });
  }
  const handleLabels: Record<ResizeHandle, string> = {
    nw: "левый верхний угол",
    n: "верхняя сторона",
    ne: "правый верхний угол",
    e: "правая сторона",
    se: "правый нижний угол",
    s: "нижняя сторона",
    sw: "левый нижний угол",
    w: "левая сторона",
  };
  const handles = () =>
    transform?.mode === "resize" && capture
      ? resizeHandles.map((handle) => {
          const r = transform.rect,
            x = handle.includes("w")
              ? r.x
              : handle.includes("e")
                ? r.x + r.width
                : r.x + r.width / 2,
            y = handle.includes("n")
              ? r.y
              : handle.includes("s")
                ? r.y + r.height
                : r.y + r.height / 2;
          return (
            <button
              key={handle}
              type="button"
              disabled={pending}
              className="grf-resize-handle bg-background text-amber-400"
              aria-label={`Изменить размер: ${handleLabels[handle]}`}
              data-resize-handle={handle}
              style={{
                left: `clamp(6px, ${(x / capture.image.width) * 100}%, calc(100% - 6px))`,
                top: `clamp(6px, ${(y / capture.image.height) * 100}%, calc(100% - 6px))`,
                cursor: handleCursor(handle),
              }}
            />
          );
        })
      : null;
  const trashIcon = (
    <svg
      width="18"
      height="18"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d="M3 6h18M9 6V3h6v3M5 6l1 15h12l1-15M10 10v7M14 10v7" />
    </svg>
  );
  const overlay = (
    r: Rect,
    label: string,
    highlight: boolean,
    annotation?: Annotation,
  ) => (
    <button
      type="button"
      disabled={(!annotation && !transform) || pending}
      tabIndex={annotation ? 0 : -1}
      data-annotation-id={annotation?.id}
      data-transform-body={
        transform && (annotation?.id ?? null) === transform.annotationId
          ? "true"
          : undefined
      }
      aria-label={annotation ? `Выбрать область ${label}` : undefined}
      aria-pressed={annotation ? highlight : undefined}
      onClick={(e) => {
        e.stopPropagation();
        if (
          !annotation ||
          transform?.annotationId === annotation.id ||
          (e.detail !== 0 &&
            (suppressRegionClick.current || space.current || mode === "pan"))
        )
          return;
        chooseAnnotation(annotation);
      }}
      className={`grf-region ${annotation || transform ? "grf-region-hit" : ""} ${highlight ? "grf-region-active border-amber-400 text-amber-400" : "border-foreground/60 text-foreground"}`}
      style={{
        left: `${(r.x / capture!.image.width) * 100}%`,
        top: `${(r.y / capture!.image.height) * 100}%`,
        width: `${(r.width / capture!.image.width) * 100}%`,
        height: `${(r.height / capture!.image.height) * 100}%`,
      }}
    >
      <span
        className={
          highlight
            ? "bg-amber-400 text-black font-bold"
            : "bg-background text-foreground border border-current"
        }
      >
        {label}
      </span>
    </button>
  );
  return (
    <div
      className="grf-workbench bg-background text-foreground"
      onKeyDown={(e) => {
        if (e.repeat || e.metaKey || e.ctrlKey || e.altKey || pending) return;
        if (
          e.target instanceof Element &&
          e.target.closest(
            "input, textarea, select, [contenteditable]:not([contenteditable='false']), [role='textbox']",
          )
        )
          return;
        if (e.key === "Escape" && transform) {
          e.preventDefault();
          e.stopPropagation();
          cancelTransform();
          return;
        }
        if (drag.current || transformDrag.current) return;
        const r = e.code === "KeyR" || e.key.toLowerCase() === "r",
          g = e.code === "KeyG" || e.key.toLowerCase() === "g";
        if ((r || g) && (selection || active || transform)) {
          e.preventDefault();
          e.stopPropagation();
          startTransform(g ? "move" : "resize");
          return;
        }
        if (e.key === "Enter" && transform) {
          e.preventDefault();
          e.stopPropagation();
          void finishTransform();
          return;
        }
        if (e.key !== "Delete" && e.key !== "Backspace") return;
        if (selection) {
          e.preventDefault();
          e.stopPropagation();
          cancelTransform();
          setSelection(null);
          setComment("");
        } else if (active) {
          e.preventDefault();
          e.stopPropagation();
          void removeAnnotation(active);
        }
      }}
    >
      <div className="grf-toolbar border-b border-border">
        <select
          aria-label="Снимок"
          className="grf-select border border-input bg-background"
          value={selected}
          disabled={pending}
          onChange={(e) => {
            setSelected(e.target.value);
            setActiveAnnotation("");
          }}
        >
          <option value="">Выберите снимок</option>
          {captures.map((c) => (
            <option key={c.id} value={c.id}>
              {c.title}
            </option>
          ))}
          {capture && !captures.some((c) => c.id === capture.id) && (
            <option value={capture.id}>{capture.title}</option>
          )}
        </select>
        <input
          ref={fileInput}
          type="file"
          accept=".png,.jpg,.jpeg,image/png,image/jpeg"
          aria-label="Выбор снимка"
          hidden
          onChange={(e) => {
            const file = e.currentTarget.files?.[0];
            e.currentTarget.value = "";
            if (!file || !activeThread || pending) return;
            const controller = new AbortController();
            uploadController.current = controller;
            void run(async () => {
              const c = await uploadPickedImage(file, rpc, {
                threadId: activeThread,
                title: title.trim(),
                sceneContext: context,
                signal: controller.signal,
                progress: (percent) =>
                  setNotice(`Загрузка снимка… ${percent}%`),
              });
              if (controller.signal.aborted) return;
              setCaptures((old) => [c, ...old]);
              setSelected(c.id);
              setActiveAnnotation("");
              setImportOpen(false);
              setPath("");
              setTitle("");
              setContext("");
              setNotice("Снимок открыт.");
              refresh();
            });
          }}
        />
        <Button
          size="sm"
          variant="outline"
          disabled={!activeThread || pending}
          onClick={() => fileInput.current?.click()}
        >
          Выбрать файл…
        </Button>
        <Button
          size="sm"
          variant="ghost"
          disabled={!activeThread || pending}
          onClick={() => setImportOpen((v) => !v)}
        >
          Указать путь
        </Button>
        {threadId && capture && (
          <Button
            size="sm"
            variant="ghost"
            onClick={() =>
              navigate.toPluginPanel("review", { subPath: capture.id })
            }
          >
            Развернуть
          </Button>
        )}
      </div>
      {(importOpen || (!capture && !selected)) && (
        <form
          className="grf-import border-b border-border"
          onSubmit={(e) => {
            e.preventDefault();
            if (!activeThread) return;
            void run(async () => {
              const c = await rpc.call("importImage", {
                threadId: activeThread,
                path,
                title:
                  title.trim() || path.split(/[\\/]/).pop() || "Снимок игры",
                sceneContext: context,
              });
              setCaptures((old) => [c, ...old]);
              setSelected(c.id);
              setActiveAnnotation("");
              setImportOpen(false);
              setPath("");
              setTitle("");
              setContext("");
              refresh();
            });
          }}
        >
          <p className="text-sm">
            Откройте снимок игрового клиента, выделите проблемную область и
            оставьте комментарий.
          </p>
          <label className="grf-label">
            Путь к PNG / JPEG в проекте
            <Input
              aria-label="Путь к снимку"
              value={path}
              onChange={(e) => setPath(e.target.value)}
              placeholder="data/Screenshots/latest.png"
              disabled={pending}
            />
          </label>
          <label className="grf-label">
            Название
            <Input
              aria-label="Название снимка"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder="Персонаж перекрывается стеной"
              disabled={pending}
            />
          </label>
          <label className="grf-label">
            Контекст сцены
            <textarea
              className="grf-textarea border border-input bg-background"
              aria-label="Контекст сцены"
              value={context}
              onChange={(e) => setContext(e.target.value)}
              placeholder="Карта, координаты, направление камеры — если известны"
              maxLength={4000}
              disabled={pending}
            />
          </label>
          <div>
            <Button
              size="sm"
              type="submit"
              disabled={!activeThread || !path.trim() || pending}
            >
              {pending ? "Открываю…" : "Открыть"}
            </Button>
            <span className="ml-3 text-xs text-muted-foreground">
              PNG/JPEG до 32 MiB · сохраняется копия
            </span>
          </div>
          {!activeThread && (
            <p className="text-sm text-muted-foreground">
              Откройте GRF Workbench из панели нужного обсуждения.
            </p>
          )}
        </form>
      )}
      {error && (
        <p role="alert" className="grf-notice text-destructive">
          {error}
        </p>
      )}
      {notice && (
        <p role="status" className="grf-notice text-muted-foreground">
          {notice}
        </p>
      )}
      {selected && !capture && !error && (
        <p className="grf-notice text-muted-foreground">Загружаю снимок…</p>
      )}
      {capture && (
        <>
          <div className="grf-toolbar border-b border-border">
            <Button
              size="sm"
              variant={mode === "select" ? "secondary" : "ghost"}
              aria-pressed={mode === "select"}
              onClick={() => setMode("select")}
            >
              Выделить область
            </Button>
            <Button
              size="sm"
              variant={mode === "pan" ? "secondary" : "ghost"}
              aria-pressed={mode === "pan"}
              onClick={() => setMode("pan")}
            >
              Перемещать
            </Button>
            <select
              aria-label="Масштаб"
              className="grf-select grf-zoom border border-input bg-background"
              value={zoom ?? "fit"}
              onChange={(e) =>
                setZoom(
                  e.target.value === "fit" ? null : Number(e.target.value),
                )
              }
            >
              {zoom !== null && ![0.5, 1, 2, 4].includes(zoom) && (
                <option value={zoom}>{Math.round(zoom * 100)}%</option>
              )}
              <option value="fit">Вписать</option>
              <option value="0.5">50%</option>
              <option value="1">1:1</option>
              <option value="2">200%</option>
              <option value="4">400%</option>
            </select>
            <span className="text-xs text-muted-foreground">
              {capture.image.width} × {capture.image.height} px
            </span>
          </div>
          <div className="grf-toolbar border-b border-border">
            <Button
              size="sm"
              variant={transform?.mode === "resize" ? "secondary" : "ghost"}
              aria-pressed={transform?.mode === "resize"}
              disabled={pending || (!selection && !active)}
              onClick={() => startTransform("resize")}
            >
              Размер (R)
            </Button>
            <Button
              size="sm"
              variant={transform?.mode === "move" ? "secondary" : "ghost"}
              aria-pressed={transform?.mode === "move"}
              disabled={pending || (!selection && !active)}
              onClick={() => startTransform("move")}
            >
              Положение (G)
            </Button>
            {transform && (
              <>
                <span role="status" className="text-sm font-medium">
                  {transform.mode === "resize"
                    ? "Размер: тяните маркеры"
                    : "Положение: перетащите рамку"}{" "}
                  · x {transform.rect.x}, y {transform.rect.y} ·{" "}
                  {transform.rect.width} × {transform.rect.height} px
                </span>
                <Button
                  size="sm"
                  disabled={pending}
                  onClick={() => void finishTransform()}
                >
                  Применить (Enter)
                </Button>
                <Button
                  size="sm"
                  variant="ghost"
                  disabled={pending}
                  onClick={cancelTransform}
                >
                  Отмена (Esc)
                </Button>
              </>
            )}
          </div>
          <p className="grf-gesture-hint text-xs text-muted-foreground">
            Колёсико — зум · Средняя кнопка / Пробел + перетаскивание —
            перемещение · Левая кнопка — выделение
          </p>
          <div className="grf-body">
            <div
              ref={viewport}
              style={
                {
                  cursor: transformCursor,
                  "--grf-transform-cursor": transformCursor,
                } as CSSProperties
              }
              className={`grf-viewport ${transformCursor ? "grf-transform-drag" : ""} bg-muted/30 ${transform ? `grf-mode-${transform.mode}` : ""} ${panning ? "grf-panning" : mode === "pan" || spaceHeld ? "grf-pan-ready" : ""}`}
              aria-label="Просмотр снимка"
              onPointerEnter={() => {
                hovered.current = true;
              }}
              onPointerLeave={() => {
                hovered.current = false;
              }}
              onPointerDown={down}
              onPointerMove={move}
              onPointerUp={up}
              onPointerCancel={() => {
                if (drag.current?.mode === "select") setSelection(null);
                drag.current = null;
                transformDrag.current = null;
                setTransformCursor(undefined);
                setPanning(false);
              }}
              onLostPointerCapture={() => {
                drag.current = null;
                transformDrag.current = null;
                setTransformCursor(undefined);
                setPanning(false);
              }}
              onAuxClick={(e) => {
                if (e.button === 1) e.preventDefault();
              }}
              onKeyDown={(e) => {
                if (e.key === "Escape" && transform) {
                  e.preventDefault();
                  e.stopPropagation();
                  cancelTransform();
                  return;
                }
                if (e.key === "Escape") {
                  drag.current = null;
                  setPanning(false);
                  setSelection(null);
                }
              }}
              tabIndex={0}
            >
              <div
                className="grf-image-wrap"
                style={{
                  width: capture.image.width * scale,
                  height: capture.image.height * scale,
                  cursor: transformCursor,
                }}
              >
                <img
                  ref={imageRef}
                  src={imageURL}
                  alt={capture.title}
                  draggable={false}
                  className="grf-image"
                />
                {annotations.map((a, i) =>
                  !a.resolved || a.id === activeAnnotation ? (
                    <div key={a.id}>
                      {overlay(
                        transform?.annotationId === a.id
                          ? transform.rect
                          : a.rect,
                        String(i + 1),
                        a.id === activeAnnotation,
                        a,
                      )}
                    </div>
                  ) : null,
                )}
                {selection &&
                  overlay(
                    transform?.annotationId === null
                      ? transform.rect
                      : selection,
                    "Новая область",
                    true,
                  )}
                {handles()}
              </div>
            </div>
            <aside className="grf-inspector border-l border-border">
              <p className="font-medium text-sm">{capture.title}</p>
              {capture.skillFrame && onStudio && (
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => onStudio(capture.skillFrame!.scene)}
                >
                  Вернуться к живой сцене
                </Button>
              )}
              {capture.skillFrame && (
                <p className="text-xs text-muted-foreground">
                  Области относятся к этому снимку. В живой сцене откроются его
                  время и ракурс.
                </p>
              )}
              {capture.skillFrame && (
                <p className="text-xs">
                  Soul Strike · tick {capture.skillFrame.tick} ·{" "}
                  {capture.skillFrame.scene.camera.yaw}° · код{" "}
                  {capture.skillFrame.rendererVersion.slice(0, 12)}
                </p>
              )}
              {capture.sceneContext && (
                <p className="text-xs text-muted-foreground whitespace-pre-wrap">
                  {capture.sceneContext}
                </p>
              )}
              {selection ? (
                <form
                  className="grf-comment"
                  onSubmit={(e) => {
                    e.preventDefault();
                    void save();
                  }}
                >
                  <p className="text-sm font-medium">Новая область</p>
                  <p
                    className="text-xs text-muted-foreground"
                    data-testid="selection-coordinates"
                  >
                    x {selection.x}, y {selection.y} · {selection.width} ×{" "}
                    {selection.height} px
                  </p>
                  <label className="grf-label">
                    Что здесь не так?
                    <textarea
                      className="grf-textarea border border-input bg-background"
                      aria-label="Комментарий к области"
                      value={comment}
                      onChange={(e) => setComment(e.target.value)}
                      maxLength={4000}
                      placeholder="Здесь NPC должен быть перед стеной, но его верхняя часть исчезает…"
                    />
                  </label>
                  <Button
                    size="sm"
                    type="submit"
                    disabled={pending || !!transform || !comment.trim()}
                  >
                    Сохранить замечание
                  </Button>
                  <Button
                    size="sm"
                    variant="ghost"
                    type="button"
                    onClick={() => {
                      cancelTransform();
                      setSelection(null);
                    }}
                  >
                    Отменить выделение
                  </Button>
                </form>
              ) : (
                <p className="text-xs text-muted-foreground">
                  Обведите область на снимке. Координаты сохраняются в исходных
                  пикселях при любом масштабе.
                </p>
              )}
              {deletedAnnotation?.captureId === capture.id && (
                <div className="text-xs text-muted-foreground" role="status">
                  Область удалена.{" "}
                  <Button
                    size="sm"
                    variant="outline"
                    disabled={pending}
                    onClick={() => void restoreAnnotation()}
                  >
                    Вернуть
                  </Button>
                </div>
              )}
              <p className="text-sm font-medium">
                Замечания · {annotations.length}
              </p>
              {annotations.length === 0 && (
                <p className="text-xs text-muted-foreground">
                  Пока нет замечаний.
                </p>
              )}
              {annotations.map((a, i) => (
                <section
                  key={a.id}
                  ref={a.id === activeAnnotation ? selectedCard : undefined}
                  className={`grf-annotation border rounded-md ${a.id === activeAnnotation ? "grf-annotation-active border-amber-400 bg-amber-400/10" : "border-border"}`}
                >
                  <button
                    type="button"
                    className="grf-annotation-select"
                    onClick={() => chooseAnnotation(a)}
                    aria-pressed={a.id === activeAnnotation}
                    aria-label={`Показать замечание ${i + 1}`}
                  >
                    <span className="text-xs text-muted-foreground">
                      #{i + 1} · {a.resolved ? "Проверено" : "Открыто"}
                    </span>
                    <span className="text-sm whitespace-pre-wrap">
                      {a.comment}
                    </span>
                    <span className="text-xs text-muted-foreground">
                      x {a.rect.x}, y {a.rect.y} · {a.rect.width} ×{" "}
                      {a.rect.height} px
                    </span>
                  </button>
                  {a.id === activeAnnotation && (
                    <label className="grf-label grf-edit-comment">
                      Комментарий к выбранной области
                      <textarea
                        aria-label="Комментарий выбранной области"
                        className="grf-textarea border border-input bg-background"
                        value={annotationComment}
                        maxLength={4000}
                        disabled={pending}
                        onChange={(e) => setAnnotationComment(e.target.value)}
                      />
                    </label>
                  )}
                  <div className="grf-annotation-actions">
                    <Button
                      size="sm"
                      variant="secondary"
                      onClick={() => addToChat(a)}
                    >
                      В обсуждение
                    </Button>
                    {a.id === activeAnnotation && (
                      <Button
                        size="sm"
                        variant="outline"
                        disabled={
                          pending ||
                          !!transform ||
                          !annotationComment.trim() ||
                          annotationComment === a.comment
                        }
                        onClick={() => void saveAnnotation(a)}
                      >
                        Сохранить комментарий
                      </Button>
                    )}
                    <Button
                      size="sm"
                      variant="ghost"
                      disabled={
                        pending ||
                        !!transform ||
                        (a.id === activeAnnotation && !annotationComment.trim())
                      }
                      onClick={() => {
                        if (a.id === activeAnnotation)
                          void saveAnnotation(a, !a.resolved);
                        else
                          void run(async () => {
                            applyAnnotation(
                              await rpc.call("resolve", {
                                annotationId: a.id,
                                revision: a.revision,
                                resolved: !a.resolved,
                              }),
                            );
                          });
                      }}
                    >
                      {a.resolved ? "Переоткрыть" : "Проверено"}
                    </Button>
                    <Button
                      size="icon"
                      variant="ghost"
                      className="h-8 w-8 text-red-500 hover:text-red-600 hover:bg-red-500/10"
                      aria-label={`Удалить область ${i + 1}`}
                      disabled={pending}
                      onClick={() => void removeAnnotation(a)}
                    >
                      {trashIcon}
                    </Button>
                  </div>
                </section>
              ))}
              {active && cropURL && (
                <div className="grf-crop">
                  <p className="text-xs text-muted-foreground">
                    Выбранная область · исходные пиксели
                  </p>
                  <img src={cropURL} alt="Фрагмент выбранного замечания" />
                </div>
              )}
              <details className="text-xs text-muted-foreground">
                <summary>Источник</summary>
                <p className="break-all">{capture.sourcePath}</p>
                <p className="break-all">SHA-256: {capture.image.digest}</p>
                <p>{capture.createdAt}</p>
              </details>
            </aside>
          </div>
        </>
      )}
    </div>
  );
}
function Workbench({
  threadId,
  captureId,
  annotationId,
  section,
  assetId,
}: {
  threadId: string | null;
  captureId?: string;
  annotationId?: string;
  section?: string;
  assetId?: string;
}) {
  const [tab, setTab] = useState(
    section === "review" || captureId || annotationId
      ? "review"
      : section === "studio"
        ? "studio"
        : "assets",
  );
  const [studioScene, setStudioScene] = useState<StudioScene>();
  const [reviewCapture, setReviewCapture] = useState(captureId);
  const [reviewAnnotation, setReviewAnnotation] = useState(annotationId);
  const [resolvedThread, setResolvedThread] = useState(threadId);
  useEffect(() => {
    if (threadId) setResolvedThread(threadId);
    if (captureId || annotationId) {
      setReviewCapture(captureId);
      setReviewAnnotation(annotationId);
      setTab("review");
    } else if (section === "assets" || assetId) setTab("assets");
    else if (section === "studio") setTab("studio");
  }, [threadId, captureId, annotationId, section, assetId]);
  return (
    <div className="grf-workbench bg-background text-foreground">
      <div className="grf-toolbar border-b border-border">
        <Button
          size="sm"
          variant={tab === "studio" ? "secondary" : "ghost"}
          onClick={() => setTab("studio")}
        >
          Skill Studio
        </Button>
        <Button
          size="sm"
          variant={tab === "assets" ? "secondary" : "ghost"}
          onClick={() => setTab("assets")}
        >
          Ресурсы GRF
        </Button>
        <Button
          size="sm"
          variant={tab === "review" ? "secondary" : "ghost"}
          onClick={() => setTab("review")}
        >
          Снимки и замечания
        </Button>
      </div>
      {tab === "studio" ? (
        <SkillStudio
          threadId={resolvedThread}
          initialScene={studioScene}
          onReview={(c) => {
            setStudioScene(c.skillFrame?.scene);
            setReviewCapture(c.id);
            setReviewAnnotation(undefined);
            setTab("review");
          }}
        />
      ) : tab === "assets" ? (
        <AssetBrowser
          threadId={resolvedThread}
          initialId={assetId}
          onReview={(c) => {
            setReviewCapture(c.id);
            setReviewAnnotation(undefined);
            setTab("review");
          }}
        />
      ) : (
        <Review
          threadId={resolvedThread}
          captureId={reviewCapture}
          annotationId={reviewAnnotation}
          onThread={setResolvedThread}
          onStudio={(scene) => {
            setStudioScene(scene);
            setTab("studio");
          }}
        />
      )}
    </div>
  );
}
function ReviewCard({
  captureId,
  annotationId,
}: {
  captureId?: string;
  annotationId?: string;
}) {
  const navigate = useBbNavigate();
  return (
    <Button
      variant="outline"
      onClick={() => {
        if (
          !navigate.openThreadPanel({
            actionId: "review",
            title: "GRF Workbench",
            params: {
              captureId: captureId ?? null,
              annotationId: annotationId ?? null,
            },
          })
        )
          navigate.toPluginPanel("review", {
            subPath: annotationId
              ? `annotation/${annotationId}`
              : (captureId ?? ""),
          });
      }}
    >
      Открыть {annotationId ? "замечание к области" : "снимок"} в GRF Workbench
    </Button>
  );
}
export default definePluginApp((app) => {
  app.slots.threadPanelAction({
    id: "review",
    title: "GRF Workbench",
    icon: "Scan",
    layout: "flush",
    component: ({ threadId, params }) => {
      const p =
        params && typeof params === "object" && !Array.isArray(params)
          ? params
          : {};
      return (
        <Workbench
          threadId={threadId}
          captureId={uuid(p.captureId)}
          annotationId={uuid(p.annotationId)}
          section={typeof p.section === "string" ? p.section : undefined}
          assetId={
            typeof p.assetId === "string" && /^[a-f0-9]{64}$/.test(p.assetId)
              ? p.assetId
              : undefined
          }
        />
      );
    },
  });
  app.slots.navPanel({
    id: "workbench",
    title: "GRF Workbench",
    path: "review",
    icon: "Scan",
    component: ({ subPath }) => {
      const parts = subPath.split("/");
      if (parts[0] === "studio")
        return <Workbench threadId={parts[1] || null} section="studio" />;
      if (parts[0] === "assets")
        return (
          <Workbench
            threadId={parts[1] || null}
            section="assets"
            assetId={parts[2]}
          />
        );
      return (
        <Workbench
          threadId={null}
          captureId={uuid(subPath)}
          annotationId={
            subPath.startsWith("annotation/")
              ? uuid(subPath.slice(11))
              : undefined
          }
        />
      );
    },
  });
  app.slots.messageDirective({
    id: "grf-resource",
    component: ({ attributes, message, source }) => {
      const navigate = useBbNavigate();
      if (!attributes.id || !/^[a-f0-9]{64}$/.test(attributes.id))
        return <span>{source}</span>;
      return (
        <Button
          variant="outline"
          onClick={() => {
            if (
              !navigate.openThreadPanel({
                actionId: "review",
                title: "Ресурсы GRF",
                params: { section: "assets", assetId: attributes.id! },
              })
            )
              navigate.toPluginPanel("review", {
                subPath: `assets/${message.threadId}/${attributes.id}`,
              });
          }}
        >
          Открыть ресурс GRF
        </Button>
      );
    },
  });
  app.slots.messageDirective({
    id: "grf-review",
    component: ({ attributes, source }) =>
      uuid(attributes.capture) || uuid(attributes.annotation) ? (
        <ReviewCard
          captureId={uuid(attributes.capture)}
          annotationId={uuid(attributes.annotation)}
        />
      ) : (
        <span>{source}</span>
      ),
  });
});
