import { useEffect, useRef, useState } from "react";
import { useBbNavigate, useRpc } from "@get-bb/plugin-sdk/app";
import type {
  Capture,
  GrfInfo,
  GrfPreview,
  GrfSearch,
  rpcContract,
} from "./contract";
import { Button } from "./components/ui/button";
import { Input } from "./components/ui/input";
import { useImagePan } from "./use-image-pan";

export function AssetBrowser({
  threadId,
  initialId,
  compact = false,
  onReview,
}: {
  threadId: string | null;
  initialId?: string;
  compact?: boolean;
  onReview: (capture: Capture) => void;
}) {
  const rpc = useRpc<typeof rpcContract>();
  const navigate = useBbNavigate();
  const [query, setQuery] = useState(""),
    [type, setType] = useState("act"),
    [archive, setArchive] = useState(-1),
    [offset, setOffset] = useState(0);
  const [results, setResults] = useState<GrfSearch | null>(null),
    [searching, setSearching] = useState(false),
    [searchError, setSearchError] = useState("");
  const [selected, setSelected] = useState(initialId ?? ""),
    [info, setInfo] = useState<GrfInfo | null>(null),
    [preview, setPreview] = useState<GrfPreview | null>(null);
  const [action, setAction] = useState(0),
    [firstFrame, setFirstFrame] = useState(0),
    [animated, setAnimated] = useState(true),
    [colorKey, setColorKey] = useState(true);
  const [frame, setFrame] = useState(0),
    [playing, setPlaying] = useState(false),
    [speed, setSpeed] = useState(1),
    [zoom, setZoom] = useState(2);
  const [image, setImage] = useState(""),
    [error, setError] = useState(""),
    [loading, setLoading] = useState(false),
    [saving, setSaving] = useState(false),
    [refresh, setRefresh] = useState(0);
  const view = useRef<HTMLDivElement>(null);
  const { spaceHeld, panning } = useImagePan(view, Boolean(preview));
  useEffect(() => {
    if (initialId) setSelected(initialId);
  }, [initialId]);
  useEffect(() => {
    if (!threadId || compact) return;
    let live = true;
    setSearching(true);
    setSearchError("");
    const timer = setTimeout(() => {
      rpc.call("grfSearch", { threadId, query, type, archive, offset }).then(
        (r) => {
          if (live) {
            setResults(r);
            setSearching(false);
          }
        },
        (e) => {
          if (live) {
            setSearchError(String(e.message ?? e));
            setSearching(false);
          }
        },
      );
    }, 250);
    return () => {
      live = false;
      clearTimeout(timer);
    };
  }, [threadId, query, type, archive, offset, rpc, refresh]);
  useEffect(() => {
    setInfo(null);
    setLoading(false);
    setPreview(null);
    setImage("");
    setPlaying(false);
    setError("");
    setAction(0);
    setFirstFrame(0);
    setFrame(0);
    if (!selected || !threadId) return;
    let live = true;
    rpc.call("grfInspect", { threadId, id: selected, archive }).then(
      (r) => {
        if (live) setInfo(r);
      },
      (e) => {
        if (live) setError(String(e.message ?? e));
      },
    );
    return () => {
      live = false;
    };
  }, [selected, archive, threadId, rpc, refresh]);
  useEffect(() => {
    if (!info || !threadId) return;
    let live = true;
    setPreview(null);
    setImage("");
    setLoading(true);
    setError("");
    setPlaying(false);
    setFrame(0);
    (async () => {
      try {
        const p = await rpc.call("grfRender", {
          threadId,
          id: info.entry.id,
          archive,
          action,
          frame: firstFrame,
          animated:
            (info.entry.type === "act" || info.entry.type === "spr") &&
            animated,
          colorKey,
        });
        let offset = 0;
        const parts: string[] = [];
        while (live) {
          const c = await rpc.call("grfImage", {
            previewId: p.previewId,
            offset,
          });
          parts.push(c.data);
          if (c.done) break;
          offset = c.nextOffset;
        }
        if (live) {
          setPreview(p);
          setImage(`data:image/png;base64,${parts.join("")}`);
          setLoading(false);
        }
      } catch (e) {
        if (live) {
          setError(String((e as Error).message ?? e));
          setLoading(false);
        }
      }
    })();
    return () => {
      live = false;
    };
  }, [info, threadId, archive, action, firstFrame, animated, colorKey, rpc]);
  useEffect(() => {
    if (!playing || !preview || preview.frameCount < 2) return;
    const start = performance.now(),
      initialFrame = frame;
    let request = 0;
    const tick = (now: number) => {
      setFrame(
        (initialFrame +
          Math.floor((now - start) / (preview.intervalMs / speed))) %
          preview.frameCount,
      );
      request = requestAnimationFrame(tick);
    };
    request = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(request);
  }, [playing, preview?.previewId, speed]);
  useEffect(() => {
    const el = view.current;
    if (!el) return;
    const wheel = (e: WheelEvent) => {
      e.preventDefault();
      if (panning) return;
      setZoom((z) =>
        Math.min(
          8,
          Math.max(
            0.1,
            z *
              Math.exp(
                -Math.max(
                  -500,
                  Math.min(500, e.deltaY * (e.deltaMode === 1 ? 16 : 1)),
                ) * 0.002,
              ),
          ),
        ),
      );
    };
    el.addEventListener("wheel", wheel, { passive: false });
    return () => el.removeEventListener("wheel", wheel);
  }, [Boolean(preview), panning]);
  if (!threadId)
    return (
      <p className="grf-notice text-muted-foreground">
        Откройте GRF Workbench из обсуждения проекта, чтобы использовать его
        архивы.
      </p>
    );
  const maxFrame =
    info?.entry.type === "act"
      ? (info.actions[action]?.frames ?? 1)
      : info?.entry.type === "spr"
        ? info.imageCount
        : 1;
  return (
    <div className={`grf-assets ${compact ? "grf-assets-embedded" : ""}`}>
      {!compact && (
        <div className="grf-toolbar border-b border-border">
          <Input
            aria-label="Поиск в GRF"
            placeholder="Имя или путь: rocker, poring, basic_interface…"
            value={query}
            onChange={(e) => {
              setQuery(e.target.value);
              setOffset(0);
            }}
          />
          <select
            aria-label="Тип ресурса"
            className="grf-select border border-input bg-background"
            value={type}
            onChange={(e) => {
              setType(e.target.value);
              setOffset(0);
            }}
          >
            <option value="">Все типы</option>
            <option value="act">Анимации ACT</option>
            <option value="spr">Спрайты SPR</option>
            <option value="bmp">BMP</option>
            <option value="tga">TGA</option>
            <option value="png">PNG</option>
            <option value="jpg">JPG</option>
          </select>
          <select
            aria-label="Архив"
            className="grf-select border border-input bg-background"
            value={archive}
            onChange={(e) => {
              setArchive(Number(e.target.value));
              setOffset(0);
            }}
          >
            <option value={-1}>Как в клиенте: приоритет архивов</option>
            {results?.archives.map((a) => (
              <option value={a.index} key={a.index}>
                {a.path.split(/[\\/]/).pop()} · {a.files.toLocaleString()}{" "}
                файлов
              </option>
            ))}
          </select>
          <Button
            size="sm"
            variant="ghost"
            onClick={() => setRefresh((n) => n + 1)}
          >
            Обновить
          </Button>
          <Button
            size="sm"
            variant="ghost"
            onClick={() =>
              navigate.toPluginPanel("review", {
                subPath: `assets/${threadId}/${selected}`,
              })
            }
          >
            Развернуть
          </Button>
        </div>
      )}
      {searchError && (
        <p role="alert" className="grf-notice text-destructive">
          {searchError}
        </p>
      )}
      <div className="grf-assets-body">
        {!compact && (
          <aside className="grf-resource-list border-r border-border">
            <p className="text-xs text-muted-foreground">
              {searching
                ? "Загружаю архивы…"
                : results
                  ? `${results.matched.toLocaleString()} совпадений · ${results.total.toLocaleString()} ресурсов`
                  : "Архивы из config.yaml"}
            </p>
            {results?.entries.map((e) => (
              <button
                type="button"
                key={e.id}
                className={`grf-resource-row ${selected === e.id ? "bg-primary/10" : "hover:bg-muted/50"}`}
                onClick={() => setSelected(e.id)}
              >
                <span className="text-sm">{e.path.split("/").pop()}</span>
                <span className="text-xs text-muted-foreground break-all">
                  {e.path}
                </span>
                <span className="text-xs text-muted-foreground">
                  {e.source.split(/[\\/]/).pop()} ·{" "}
                  {(e.bytes / 1024).toFixed(1)} KiB
                  {e.variants.length > 1
                    ? ` · ${e.variants.length} версии`
                    : ""}
                </span>
              </button>
            ))}
            <div className="grf-toolbar">
              <Button
                size="sm"
                variant="outline"
                disabled={offset === 0 || searching}
                onClick={() => setOffset(Math.max(0, offset - 50))}
              >
                Назад
              </Button>
              <Button
                size="sm"
                variant="outline"
                disabled={!results || results.next < 0 || searching}
                onClick={() => setOffset(results!.next)}
              >
                Далее
              </Button>
            </div>
          </aside>
        )}
        <section className="grf-resource-detail">
          {!selected && (
            <p className="text-sm text-muted-foreground">
              Выберите ресурс. Для настоящей анимации откройте ACT; SPR содержит
              исходные изображения кадров.
            </p>
          )}
          {info && (
            <>
              <div className="grf-resource-heading">
                <p className="text-sm font-medium break-all">
                  {info.entry.path}
                </p>
                <p className="text-xs text-muted-foreground break-all">
                  {info.entry.source}
                  {info.version
                    ? ` · ${info.entry.type.toUpperCase()} ${info.version}`
                    : ""}
                </p>
              </div>
              {info.pair && (
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => setSelected(info.pair!.id)}
                >
                  Открыть {info.pair.type.toUpperCase()}:{" "}
                  {info.pair.path.split("/").pop()}
                </Button>
              )}
              <div className="grf-toolbar">
                {info.actions.length > 0 && (
                  <select
                    aria-label="Действие ACT"
                    className="grf-select border border-input bg-background"
                    value={action}
                    onChange={(e) => {
                      setAction(Number(e.target.value));
                      setFirstFrame(0);
                    }}
                  >
                    {info.actions.map((a) => (
                      <option key={a.index} value={a.index}>
                        {a.index}: {a.name} · {a.frames} кадров
                      </option>
                    ))}
                  </select>
                )}
                {(info.entry.type === "act" || info.entry.type === "spr") && (
                  <>
                    <label className="text-xs">
                      С кадра{" "}
                      <input
                        className="grf-frame-number border border-input bg-background"
                        type="number"
                        min={0}
                        max={Math.max(0, maxFrame - 1)}
                        aria-label="Начальный кадр"
                        value={firstFrame}
                        onChange={(e) => {
                          const n = Number(e.target.value);
                          if (Number.isInteger(n) && n >= 0 && n < maxFrame)
                            setFirstFrame(n);
                        }}
                      />
                    </label>
                    <label className="text-xs">
                      <input
                        type="checkbox"
                        checked={animated}
                        onChange={(e) => setAnimated(e.target.checked)}
                      />{" "}
                      Загрузить последовательность
                    </label>
                  </>
                )}
                {!["spr", "act"].includes(info.entry.type) && (
                  <label className="text-xs">
                    <input
                      type="checkbox"
                      checked={colorKey}
                      onChange={(e) => setColorKey(e.target.checked)}
                    />{" "}
                    Прозрачность magenta как в клиенте
                  </label>
                )}
              </div>
            </>
          )}
          {error && (
            <p role="alert" className="grf-notice text-destructive">
              {error}
            </p>
          )}
          {loading && (
            <p className="text-sm text-muted-foreground">
              Готовлю превью ресурса…
            </p>
          )}
          {preview && (
            <>
              <div className="grf-toolbar">
                <Button
                  size="sm"
                  disabled={preview.frameCount < 2}
                  onClick={() => setPlaying((p) => !p)}
                >
                  {playing ? "Пауза" : "Воспроизвести"}
                </Button>
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={() => {
                    setPlaying(false);
                    setFrame((f) => Math.max(0, f - 1));
                  }}
                >
                  ◀
                </Button>
                <input
                  aria-label="Кадр анимации"
                  type="range"
                  min={0}
                  max={preview.frameCount - 1}
                  value={frame}
                  onChange={(e) => {
                    setPlaying(false);
                    setFrame(Number(e.target.value));
                  }}
                />
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={() => {
                    setPlaying(false);
                    setFrame((f) => Math.min(preview.frameCount - 1, f + 1));
                  }}
                >
                  ▶
                </Button>
                <span className="text-xs">
                  Кадр {preview.firstFrame + frame} · {preview.width} ×{" "}
                  {preview.height} px
                </span>
                <select
                  aria-label="Скорость анимации"
                  className="grf-select border border-input bg-background"
                  value={speed}
                  onChange={(e) => setSpeed(Number(e.target.value))}
                >
                  <option value={0.5}>0.5×</option>
                  <option value={1}>1×</option>
                  <option value={2}>2×</option>
                </select>
                <Button size="sm" variant="ghost" onClick={() => setZoom(1)}>
                  1:1
                </Button>
                <span className="text-xs text-muted-foreground">
                  {Math.round(zoom * 100)}%
                </span>
              </div>
              <div
                ref={view}
                tabIndex={0}
                aria-label="Просмотр ресурса GRF"
                className={`grf-resource-view bg-muted/30 ${panning ? "grf-panning" : spaceHeld ? "grf-pan-ready" : ""}`}
              >
                <div
                  className="grf-resource-pixels"
                  role="img"
                  aria-label={`Превью ${preview.info.entry.path}, кадр ${preview.firstFrame + frame}`}
                  style={{
                    width: preview.width * zoom,
                    height: preview.height * zoom,
                    backgroundImage: `url("${image}")`,
                    backgroundSize: `${preview.image.width * zoom}px ${preview.image.height * zoom}px`,
                    backgroundPosition: `${-(frame % preview.columns) * preview.width * zoom}px ${-Math.floor(frame / preview.columns) * preview.height * zoom}px`,
                  }}
                />
              </div>
              <div className="grf-toolbar">
                <Button
                  size="sm"
                  disabled={saving}
                  onClick={async () => {
                    setPlaying(false);
                    setSaving(true);
                    try {
                      onReview(
                        await rpc.call("grfCapture", {
                          previewId: preview.previewId,
                          frame,
                        }),
                      );
                    } catch (e) {
                      setError(String((e as Error).message ?? e));
                    } finally {
                      setSaving(false);
                    }
                  }}
                >
                  {saving
                    ? "Сохраняю кадр…"
                    : "Выделить область и оставить замечание"}
                </Button>
              </div>
              <p className="text-xs text-muted-foreground">
                {preview.info.entry.type === "act"
                  ? `${preview.intervalMs} мс/кадр · тайминг клиента`
                  : preview.info.entry.type === "spr"
                    ? "Лист исходных изображений SPR; для анимации откройте парный ACT."
                    : "Декодер изображения из клиента"}{" "}
                · origin ({preview.originX}, {preview.originY}) · колёсико —
                масштаб
              </p>
              {preview.info.warnings.map((w) => (
                <p key={w} className="text-sm text-muted-foreground">
                  {w}
                </p>
              ))}
              <details className="text-xs text-muted-foreground">
                <summary>Происхождение и рендер</summary>
                <p>{preview.renderer}</p>
                {preview.dependencies.map((d) => (
                  <p className="break-all" key={d.path}>
                    {d.path} · {d.source} · SHA-256 {d.sha256}
                  </p>
                ))}
              </details>
            </>
          )}
        </section>
      </div>
    </div>
  );
}
