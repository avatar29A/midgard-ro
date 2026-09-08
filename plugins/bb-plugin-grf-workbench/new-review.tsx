import { useEffect, useRef, useState } from "react";
import {
  experimental_NewThreadComposer as NewThreadComposer,
  useBbNavigate,
  useRpc,
  type NewThreadRequest,
} from "@get-bb/plugin-sdk/app";
import type { Capture, Annotation, ReviewDraft, rpcContract } from "./contract";
import { Button } from "./components/ui/button";
import { Input } from "./components/ui/input";

const pendingOpens = new Map<string, string>();
const openingKey = (threadId: string) => `grf-review-open:${threadId}`;
function seed(d: ReviewDraft) {
  const comments = d.annotations
    .map(
      (a, i) =>
        `${i + 1}. ${a.comment.slice(0, 500)}${a.comment.length > 500 ? "…" : ""}`,
    )
    .join("\n");
  return `Разбери визуальные замечания к снимку «${d.capture.title}». Сначала посмотри изображение и отмеченные области, затем подготовь исправление и воспроизводимый кадр для проверки. Эталон исполнения — текущий игровой клиент midgard-ro.\n\n${comments || "Опиши здесь, что нужно проверить на снимке."}`;
}
export function NewReview({
  captureId,
  operationId,
}: {
  captureId?: string;
  operationId?: string;
}) {
  const rpc = useRpc<typeof rpcContract>(),
    navigate = useBbNavigate();
  const [source, setSource] = useState<Capture | null>(null),
    [annotations, setAnnotations] = useState<Annotation[]>([]);
  const [selected, setSelected] = useState<string[]>([]),
    [title, setTitle] = useState("");
  const [draft, setDraft] = useState<ReviewDraft | null>(null),
    [error, setError] = useState("");
  const [busy, setBusy] = useState(false),
    [image, setImage] = useState("");
  const intent = useRef(crypto.randomUUID());
  useEffect(() => {
    let live = true;
    setError("");
    setDraft(null);
    setSource(null);
    if (operationId) {
      rpc.call("reviewDraft", { operationId }).then(
        (d) => {
          if (live) setDraft(d);
        },
        (e) => {
          if (live) setError(e.message);
        },
      );
    } else if (captureId) {
      rpc
        .call("get", { captureId })
        .then(async (result) => {
          if (!live) return;
          setSource(result.capture);
          setAnnotations(result.annotations);
          setSelected(
            result.annotations.filter((a) => !a.resolved).map((a) => a.id),
          );
          setTitle(`Разбор: ${result.capture.title}`.slice(0, 200));
          const chunks: string[] = [];
          let offset = 0,
            mime = "image/png";
          while (live) {
            const part = await rpc.call("image", { captureId, offset });
            chunks.push(part.data);
            mime = part.mimeType;
            if (part.done) break;
            offset = part.nextOffset;
          }
          if (live) setImage(`data:${mime};base64,${chunks.join("")}`);
        })
        .catch((e) => {
          if (live) setError(String(e.message ?? e));
        });
    }
    return () => {
      live = false;
    };
  }, [captureId, operationId, rpc]);
  function open(threadId: string, copyId: string) {
    pendingOpens.set(threadId, copyId);
    try {
      sessionStorage.setItem(openingKey(threadId), copyId);
    } catch {}
    navigate.toThread(threadId);
  }
  async function start(request?: NewThreadRequest) {
    if (!draft) return;
    setBusy(true);
    setError("");
    try {
      const result = await rpc.call("startReview", {
        operationId: draft.id,
        ...(request ? { request } : {}),
      });
      open(result.threadId, result.captureId);
    } catch (e) {
      setError(String((e as Error).message ?? e));
      try {
        setDraft(await rpc.call("reviewDraft", { operationId: draft.id }));
      } catch {}
      throw e; // BB's composer retains the unsent draft.
    } finally {
      setBusy(false);
    }
  }
  return (
    <div className="grf-new-review bg-background text-foreground">
      <div className="grf-toolbar border-b border-border">
        <strong>Новый разбор</strong>
        <Button
          size="sm"
          variant="ghost"
          disabled={busy}
          onClick={() =>
            navigate.toPluginPanel("review", {
              subPath: captureId ?? draft?.capture.copiedFrom?.captureId ?? "",
            })
          }
        >
          К исходному снимку
        </Button>
      </div>
      {busy && (
        <p role="status" className="grf-notice text-sm">
          Подготавливаю разбор…
        </p>
      )}
      {error && (
        <p role="alert" className="grf-notice text-destructive">
          {error}
        </p>
      )}
      {!source && !draft && !error && (
        <p className="grf-notice">Загружаю контекст снимка…</p>
      )}
      {source && (
        <div className="grf-new-review-selection">
          <section>
            <p className="text-sm text-muted-foreground">
              Выбранные замечания получат независимую копию в новом треде. На
              следующем шаге можно отредактировать запрос и выбрать модель.
            </p>
            <label className="grf-label">
              Название разбора
              <Input
                aria-label="Название разбора"
                value={title}
                maxLength={200}
                disabled={busy}
                onChange={(e) => setTitle(e.target.value)}
              />
            </label>
            <div className="grf-toolbar">
              <Button
                size="sm"
                variant="ghost"
                disabled={busy}
                onClick={() =>
                  setSelected(
                    annotations.filter((a) => !a.resolved).map((a) => a.id),
                  )
                }
              >
                Непроверенные
              </Button>
              <Button
                size="sm"
                variant="ghost"
                disabled={busy}
                onClick={() => setSelected(annotations.map((a) => a.id))}
              >
                Все
              </Button>
              <Button
                size="sm"
                variant="ghost"
                disabled={busy}
                onClick={() => setSelected([])}
              >
                Без замечаний
              </Button>
            </div>
            {annotations.map((a, i) => (
              <label
                className="grf-new-review-comment border border-border"
                key={a.id}
              >
                <input
                  type="checkbox"
                  aria-label={`Включить замечание ${i + 1}`}
                  checked={selected.includes(a.id)}
                  disabled={busy}
                  onChange={(e) =>
                    setSelected((old) =>
                      e.target.checked
                        ? [...old, a.id]
                        : old.filter((id) => id !== a.id),
                    )
                  }
                />
                <span>
                  <strong>
                    #{i + 1}
                    {a.resolved ? " · Проверено" : ""}
                  </strong>
                  <span className="grf-review-comment-body">{a.comment}</span>
                </span>
              </label>
            ))}
            {!annotations.length && (
              <p className="text-sm text-muted-foreground">
                Замечаний пока нет — можно начать разбор всего снимка.
              </p>
            )}
            <Button
              disabled={busy || !title.trim()}
              onClick={async () => {
                setBusy(true);
                setError("");
                try {
                  const d = await rpc.call("prepareReview", {
                    operationId: intent.current,
                    captureId: source.id,
                    imageDigest: source.image.digest,
                    title: title.trim(),
                    annotations: annotations
                      .filter((a) => selected.includes(a.id))
                      .map((a) => ({ id: a.id, revision: a.revision })),
                  });
                  navigate.toPluginPanel("review", {
                    subPath: `draft/${d.id}`,
                    replace: true,
                  });
                } catch (e) {
                  setError(String((e as Error).message ?? e));
                } finally {
                  setBusy(false);
                }
              }}
            >
              {busy
                ? "Готовлю копию…"
                : `Подготовить запрос · ${selected.length} замечаний`}
            </Button>
          </section>
          <aside>
            <p className="text-sm">{source.title}</p>
            {image && (
              <div className="grf-review-selection-image">
                <img src={image} alt={source.title} />
                <svg
                  viewBox={`0 0 ${source.image.width} ${source.image.height}`}
                  aria-hidden="true"
                >
                  {annotations.map((a, i) => (
                    <g key={a.id} opacity={selected.includes(a.id) ? 1 : 0.4}>
                      <rect
                        x={a.rect.x}
                        y={a.rect.y}
                        width={a.rect.width}
                        height={a.rect.height}
                        fill="none"
                        stroke={selected.includes(a.id) ? "#fbbf24" : "#ffffff"}
                        strokeWidth={2}
                        vectorEffect="non-scaling-stroke"
                      />
                      <text
                        x={a.rect.x + 2}
                        y={Math.min(
                          source.image.height - 2,
                          a.rect.y + Math.max(16, source.image.width / 35),
                        )}
                        fontSize={Math.max(16, source.image.width / 35)}
                        fill="#fbbf24"
                        stroke="#171717"
                        strokeWidth={1}
                        paintOrder="stroke"
                      >
                        {i + 1}
                      </text>
                    </g>
                  ))}
                </svg>
              </div>
            )}
          </aside>
        </div>
      )}
      {draft && (
        <section className="grf-review-composer">
          <h2 className="font-medium">{draft.title}</h2>
          {draft.status === "prepared" && (
            <Button
              size="sm"
              variant="ghost"
              disabled={busy}
              onClick={async () => {
                setBusy(true);
                setError("");
                try {
                  await rpc.call("discardReviewDraft", {
                    operationId: draft.id,
                  });
                  navigate.toPluginPanel("review", {
                    subPath: draft.capture.copiedFrom?.captureId ?? "",
                  });
                } catch (e) {
                  setError(String((e as Error).message ?? e));
                } finally {
                  setBusy(false);
                }
              }}
            >
              Отменить разбор
            </Button>
          )}
          <p className="text-sm text-muted-foreground">
            Снимок и {draft.annotations.length} замечаний зафиксированы для
            этого разбора. Новый агент получит изображение, координаты областей
            и контекст кадра. Настройки ниже принадлежат новому треду.
          </p>
          {draft.status === "prepared" ? (
            <NewThreadComposer
              key={draft.id}
              draftKey={`grf-review:${draft.id}`}
              defaultProjectId={draft.capture.projectId}
              defaultEnvironment={draft.defaultEnvironment}
              initialPrompt={seed(draft)}
              layout="document"
              onSubmit={start}
            />
          ) : (
            <div className="grf-toolbar">
              <Button
                disabled={busy}
                onClick={() => {
                  void start().catch(() => {});
                }}
              >
                {busy
                  ? "Проверяю…"
                  : draft.status === "done"
                    ? "Открыть разбор"
                    : "Продолжить / проверить создание"}
              </Button>
              {draft.threadId && (
                <Button
                  variant="outline"
                  disabled={busy}
                  onClick={() => open(draft.threadId!, draft.capture.id)}
                >
                  Открыть созданный тред
                </Button>
              )}
            </div>
          )}
        </section>
      )}
    </div>
  );
}

// Runs in the destination thread's own surface, after navigation. BB owns the
// split, resizing and persistence. No DOM navigation or private tab store.
export function ReviewThreadHeader({
  threadId,
  isCompactViewport,
}: {
  threadId: string;
  isCompactViewport: boolean;
}) {
  const rpc = useRpc<typeof rpcContract>(),
    navigate = useBbNavigate();
  const nav = useRef(navigate);
  nav.current = navigate;
  const [review, setReview] = useState<{
    captureId: string;
    title: string;
  } | null>(null);
  useEffect(() => {
    let live = true;
    let timer: ReturnType<typeof setTimeout> | undefined;
    rpc
      .call("reviewForThread", { threadId })
      .then((r) => {
        if (!live) return;
        setReview(r);
        if (!r) return;
        let pending: string | null = pendingOpens.get(threadId) ?? null;
        try {
          pending = sessionStorage.getItem(openingKey(threadId)) ?? pending;
        } catch {}
        if (pending !== r.captureId) return;
        let attempts = 0;
        const reveal = () => {
          if (!live) return;
          if (
            nav.current.openThreadPanel({
              actionId: "review",
              title: r.title,
              params: { captureId: r.captureId, section: "review" },
            })
          ) {
            pendingOpens.delete(threadId);
            try {
              sessionStorage.removeItem(openingKey(threadId));
            } catch {}
          } else if (++attempts < 3) timer = setTimeout(reveal, 100);
        };
        timer = setTimeout(reveal, 0);
      })
      .catch(() => {});
    return () => {
      live = false;
      if (timer) clearTimeout(timer);
    };
  }, [threadId, rpc]);
  if (!review) return null;
  return (
    <Button
      size="sm"
      variant="ghost"
      aria-label="Открыть снимок разбора"
      onClick={() => {
        if (
          !navigate.openThreadPanel({
            actionId: "review",
            title: review.title,
            params: { captureId: review.captureId, section: "review" },
          })
        )
          navigate.toPluginPanel("review", { subPath: review.captureId });
      }}
    >
      {isCompactViewport ? "▧" : "Снимок разбора"}
    </Button>
  );
}
