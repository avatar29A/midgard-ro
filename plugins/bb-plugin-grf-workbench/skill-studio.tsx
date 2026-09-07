import { memo, useEffect, useRef, useState } from "react";
import { useBbNavigate, useRpc } from "@get-bb/plugin-sdk/app";
import type { Capture, rpcContract } from "./contract";
import {
  defaultStudioScene,
  type StudioFrame,
  type StudioScene,
} from "./studio-contract";
import { Button } from "./components/ui/button";

const clamp = (x: number, min: number, max: number) =>
  Math.max(min, Math.min(max, x));
const yaw = (x: number) => ((x % 360) + 360) % 360;
function CameraModeIcon({ mode }: { mode: "orbit" | "pan" }) {
  return (
    <svg
      aria-hidden="true"
      width="16"
      height="16"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      {mode === "orbit" ? (
        <path d="M3 11a9 9 0 1 1 2.5 6.2M3 4v7h7" />
      ) : (
        <path d="M12 3v18M3 12h18M8 7l4-4 4 4M8 17l4 4 4-4M7 8l-4 4 4 4M17 8l4 4-4 4" />
      )}
    </svg>
  );
}

// Camera events should not rebuild a large data URL or touch the displayed
// image until a decoded native frame is ready.
const StudioImage = memo(function StudioImage({
  frame,
}: {
  frame: StudioFrame | null;
}) {
  return frame ? (
    <img
      src={`data:image/png;base64,${frame.png}`}
      alt={`Soul Strike · tick ${frame.context.tick} · ${frame.context.scene.camera.yaw}°`}
      draggable={false}
    />
  ) : (
    <div className="grf-studio-placeholder">Mage → Soul Strike → Rocker</div>
  );
});

export function SkillStudio({
  threadId,
  initialScene,
  onReview,
}: {
  threadId: string | null;
  initialScene?: StudioScene;
  onReview: (c: Capture) => void;
}) {
  const rpc = useRpc<typeof rpcContract>(),
    navigate = useBbNavigate();
  const [scene, setScene] = useState<StudioScene>(
    initialScene ?? defaultStudioScene,
  );
  const [sessionId, setSessionId] = useState(() => crypto.randomUUID());
  const [frame, setFrame] = useState<StudioFrame | null>(null),
    [playing, setPlaying] = useState(false),
    [speed, setSpeed] = useState(1);
  const [error, setError] = useState(""),
    [loading, setLoading] = useState(true),
    [saving, setSaving] = useState(false),
    [latency, setLatency] = useState(0);
  const [mode, setMode] = useState<"orbit" | "pan">("orbit"),
    [space, setSpace] = useState(false);
  const view = useRef<HTMLDivElement>(null),
    drag = useRef<{ x: number; y: number; mode: string } | null>(null);
  const current = useRef(scene),
    play = useRef(false),
    rate = useRef(speed),
    frozen = useRef(false),
    shown = useRef<StudioFrame | null>(null);
  const serial = useRef(0);
  const wake = useRef<() => void>(() => {});
  const resetClock = useRef<() => void>(() => {});
  function change(next: StudioScene) {
    current.current = next;
    serial.current++;
    setScene(next);
    wake.current();
  }
  function togglePlay(value: boolean) {
    if (play.current !== value) resetClock.current();
    play.current = value;
    setPlaying(value);
    wake.current();
  }
  rate.current = speed;
  useEffect(() => {
    if (!threadId) return;
    let alive = true,
      inFlight = false,
      dirty = false;
    let raf: number | undefined;
    let timer: ReturnType<typeof setTimeout> | undefined;
    let heartbeat: ReturnType<typeof setTimeout> | undefined;
    let sent = "",
      lastSent = 0,
      lastTime = performance.now(),
      fraction = 0;
    setLoading(true);
    setError("");
    setFrame(null);
    shown.current = null;
    resetClock.current = () => {
      lastTime = performance.now();
      fraction = 0;
    };
    const requestFrame = () => {
      if (!alive || frozen.current) return;
      dirty = true;
      if (heartbeat !== undefined) {
        clearTimeout(heartbeat);
        heartbeat = undefined;
      }
      if (inFlight || raf !== undefined || timer !== undefined) return;
      // Coalesce input received within one display refresh. After an in-flight
      // frame completes, immediately request the latest state without this wait.
      raf = requestAnimationFrame(() => {
        raf = undefined;
        void step();
      });
    };
    wake.current = requestFrame;
    async function step() {
      if (!alive || frozen.current) return;
      inFlight = true;
      dirty = false;
      try {
        const now = performance.now(),
          elapsed = now - lastTime;
        lastTime = now;
        if (play.current) {
          fraction += ((elapsed * 60) / 1000) * rate.current;
          const n = Math.floor(fraction);
          fraction -= n;
          const end = shown.current?.context.durationTicks ?? 120;
          if (n > 0) {
            current.current = {
              ...current.current,
              tick: (current.current.tick + n) % (end + 1),
            };
            setScene(current.current);
          }
        } else fraction = 0;
        const request = structuredClone(current.current);
        const key = JSON.stringify([request, serial.current]);
        if (key !== sent || now - lastSent >= 30000) {
          const start = performance.now();
          const result = await rpc.call("studioRender", {
            threadId: threadId!,
            sessionId,
            scene: request,
          });
          if (!alive) return;
          const img = new Image();
          img.src = `data:image/png;base64,${result.png}`;
          await img.decode();
          if (!alive) return;
          // One request is in flight, so replies are ordered. New camera input
          // does not invalidate this completed frame: present it and render the
          // latest camera next. Discarding it would starve a continuous drag.
          if (!frozen.current) {
            setFrame(result);
            shown.current = result;
            setLatency(Math.round(performance.now() - start));
            setLoading(false);
            setError("");
          }
          sent = key;
          lastSent = performance.now();
        }
      } catch (e) {
        if (alive) {
          setError(String((e as Error).message ?? e));
          setLoading(false);
          play.current = false;
          setPlaying(false);
        }
        return;
      } finally {
        inFlight = false;
      }
      if (!alive || frozen.current) return;
      if (dirty || play.current) {
        // No artificial 40ms idle delay; camera interaction and playback share
        // a 60Hz budget, bounded by actual render/decode speed, with no queue.
        timer = setTimeout(
          () => {
            timer = undefined;
            void step();
          },
          Math.max(0, 1000 / 60 - (performance.now() - lastTime)),
        );
      } else {
        heartbeat = setTimeout(() => {
          heartbeat = undefined;
          requestFrame();
        }, 30000);
      }
    }
    void step();
    return () => {
      alive = false;
      wake.current = () => {};
      resetClock.current = () => {};
      if (raf !== undefined) cancelAnimationFrame(raf);
      if (timer !== undefined) clearTimeout(timer);
      if (heartbeat !== undefined) clearTimeout(heartbeat);
      void rpc.call("studioClose", { threadId, sessionId }).catch(() => {});
    };
  }, [rpc, threadId, sessionId]);
  useEffect(() => {
    const el = view.current;
    if (!el) return;
    const wheel = (e: WheelEvent) => {
      e.preventDefault();
      const c = current.current;
      change({
        ...c,
        camera: {
          ...c.camera,
          distance: clamp(
            c.camera.distance * Math.exp(clamp(e.deltaY, -300, 300) * 0.002),
            100,
            800,
          ),
        },
      });
    };
    el.addEventListener("wheel", wheel, { passive: false });
    return () => el.removeEventListener("wheel", wheel);
  }, []);
  if (!threadId)
    return (
      <p className="grf-notice">Откройте Skill Studio из обсуждения проекта.</p>
    );
  const c = frame?.context;
  function stepTick(delta: number) {
    togglePlay(false);
    change({
      ...current.current,
      tick: clamp(current.current.tick + delta, 0, c?.durationTicks ?? 120),
    });
  }
  function cameraChange(p: Partial<StudioScene["camera"]>) {
    change({ ...current.current, camera: { ...current.current.camera, ...p } });
  }
  return (
    <div className="grf-studio">
      <div className="grf-toolbar border-b border-border">
        <strong>Soul Strike</strong>
        <span className="text-xs text-muted-foreground">
          Маг · процедурный · этап 1
        </span>
        <Button
          size="sm"
          variant="ghost"
          onClick={() =>
            navigate.toPluginPanel("review", { subPath: `studio/${threadId}` })
          }
        >
          Развернуть
        </Button>
        <Button
          size="sm"
          variant="ghost"
          disabled={saving || !!error}
          onClick={() => {
            serial.current++;
            wake.current();
          }}
        >
          Обновить параметры
        </Button>
        <Button
          size="sm"
          variant="outline"
          disabled={saving}
          onClick={() => {
            togglePlay(false);
            frozen.current = false;
            serial.current++;
            setSessionId(crypto.randomUUID());
          }}
        >
          Пересобрать / загрузить ресурсы
        </Button>
      </div>
      <p className="grf-notice text-xs text-muted-foreground">
        Полёт, хвосты и вспышки попаданий. Mage и Rocker стоят в idle. Каст,
        звук и реакция цели — следующий этап.
      </p>
      <div className="grf-toolbar">
        <label>
          Уровень{" "}
          <select
            aria-label="Уровень навыка"
            value={scene.level}
            onChange={(e) =>
              change({
                ...current.current,
                level: Number(e.target.value),
                hits: 0,
                tick: 0,
              })
            }
          >
            {Array.from({ length: 10 }, (_, i) => (
              <option key={i} value={i + 1}>
                {i + 1}
              </option>
            ))}
          </select>
        </label>
        <label>
          Попадания{" "}
          <select
            aria-label="Входное число попаданий"
            value={scene.hits}
            onChange={(e) =>
              change({
                ...current.current,
                hits: Number(e.target.value),
                tick: 0,
              })
            }
          >
            <option value={0}>Из таблицы уровня</option>
            {[1, 2, 3, 4, 5, 10].map((n) => (
              <option key={n} value={n}>
                {n} · тестовый вход
              </option>
            ))}
          </select>
        </label>
        <label>
          Дистанция{" "}
          <input
            aria-label="Расстояние в клетках"
            type="number"
            min={1}
            max={30}
            value={scene.separation}
            onChange={(e) => {
              const n = Number(e.target.value);
              if (n >= 1 && n <= 30)
                change({ ...current.current, separation: n });
            }}
          />{" "}
          клеток
        </label>
      </div>
      <div className="grf-toolbar">
        <Button
          size="sm"
          disabled={!frame || saving || !!error}
          onClick={() => togglePlay(!playing)}
        >
          {playing ? "Пауза" : "Play"}
        </Button>
        <Button
          size="sm"
          variant="outline"
          aria-label="Предыдущий tick"
          disabled={saving}
          onClick={() => stepTick(-1)}
        >
          ◀
        </Button>
        <input
          aria-label="Время Soul Strike"
          type="range"
          min={0}
          max={c?.durationTicks ?? 120}
          value={scene.tick}
          disabled={saving}
          onChange={(e) => {
            togglePlay(false);
            change({ ...current.current, tick: Number(e.target.value) });
          }}
        />
        <Button
          size="sm"
          variant="outline"
          aria-label="Следующий tick"
          disabled={saving}
          onClick={() => stepTick(1)}
        >
          ▶
        </Button>
        <span className="text-xs">
          tick {c?.tick ?? "…"} · {Math.round(c?.timeMs ?? 0)} мс
        </span>
        <select
          aria-label="Скорость просмотра"
          value={speed}
          onChange={(e) => setSpeed(Number(e.target.value))}
        >
          {[0.25, 0.5, 1, 2].map((n) => (
            <option key={n} value={n}>
              {n}×
            </option>
          ))}
        </select>
      </div>
      <div className="grf-toolbar">
        <Button
          size="sm"
          variant={mode === "orbit" ? "secondary" : "ghost"}
          aria-pressed={mode === "orbit"}
          aria-description="Средняя кнопка мыши или перетаскивание в режиме вращения"
          onClick={() => setMode("orbit")}
        >
          <CameraModeIcon mode="orbit" /> Вращать
        </Button>
        <Button
          size="sm"
          variant={mode === "pan" ? "secondary" : "ghost"}
          aria-pressed={mode === "pan"}
          aria-description="Space + левая кнопка или Shift + средняя кнопка"
          onClick={() => setMode("pan")}
        >
          <CameraModeIcon mode="pan" /> Перемещать
        </Button>
        <Button
          size="sm"
          variant="ghost"
          onClick={() =>
            cameraChange({ yaw: yaw(current.current.camera.yaw - 45) })
          }
        >
          ↶ 45°
        </Button>
        <Button
          size="sm"
          variant="ghost"
          onClick={() =>
            cameraChange({ yaw: yaw(current.current.camera.yaw + 45) })
          }
        >
          45° ↷
        </Button>
        <select
          aria-label="Центр вращения"
          value={scene.camera.focus}
          onChange={(e) =>
            cameraChange({
              focus: e.target.value as StudioScene["camera"]["focus"],
            })
          }
        >
          <option value="center">Центр действия</option>
          <option value="caster">Mage</option>
          <option value="target">Rocker</option>
        </select>
        <Button
          size="sm"
          variant="ghost"
          onClick={() => cameraChange(defaultStudioScene.camera)}
        >
          Игровой ракурс
        </Button>
        <span className="text-xs text-muted-foreground">
          {Math.round(scene.camera.yaw)}° · колесо — zoom · Space + drag — pan
        </span>
      </div>
      {error && (
        <p role="alert" className="grf-notice text-destructive">
          {error} Предыдущий кадр устарел. После исправления нажмите
          «Пересобрать».
        </p>
      )}
      {loading && (
        <p role="status" className="grf-notice">
          Собираю игровой worker и загружаю GRF… Первый запуск может занять до
          двух минут.
        </p>
      )}
      <div
        ref={view}
        className="grf-studio-view"
        tabIndex={0}
        aria-label="Сцена Skill Studio"
        style={{ cursor: space || mode === "pan" ? "grab" : "ew-resize" }}
        onKeyDown={(e) => {
          if (e.code === "Space") {
            e.preventDefault();
            setSpace(true);
          }
        }}
        onKeyUp={(e) => {
          if (e.code === "Space") {
            e.preventDefault();
            setSpace(false);
          }
        }}
        onBlur={() => {
          setSpace(false);
          drag.current = null;
        }}
        onPointerDown={(e) => {
          if (saving || ![0, 1].includes(e.button)) return;
          e.preventDefault();
          e.currentTarget.focus();
          e.currentTarget.setPointerCapture(e.pointerId);
          drag.current = {
            x: e.clientX,
            y: e.clientY,
            mode: space || e.shiftKey ? "pan" : e.button === 1 ? "orbit" : mode,
          };
        }}
        onPointerMove={(e) => {
          const d = drag.current;
          if (!d) return;
          const dx = e.clientX - d.x,
            dy = e.clientY - d.y;
          d.x = e.clientX;
          d.y = e.clientY;
          const cam = current.current.camera;
          if (d.mode === "orbit")
            cameraChange({ yaw: yaw(cam.yaw - dx * 0.3) });
          else {
            const angle = (cam.yaw * Math.PI) / 180,
              scale = cam.distance / 500;
            cameraChange({
              panX: clamp(
                cam.panX +
                  (dx * Math.cos(angle) + dy * Math.sin(angle)) * scale,
                -200,
                200,
              ),
              panZ: clamp(
                cam.panZ +
                  (-dx * Math.sin(angle) + dy * Math.cos(angle)) * scale,
                -200,
                200,
              ),
            });
          }
        }}
        onPointerUp={(e) => {
          drag.current = null;
          if (e.currentTarget.hasPointerCapture(e.pointerId))
            e.currentTarget.releasePointerCapture(e.pointerId);
        }}
        onPointerCancel={() => {
          drag.current = null;
        }}
      >
        <StudioImage frame={frame} />
      </div>
      <div className="grf-toolbar">
        <Button
          size="sm"
          disabled={!frame || saving || !!error}
          onClick={async () => {
            const displayed = frame!;
            togglePlay(false);
            frozen.current = true;
            setSaving(true);
            try {
              const capture = await rpc.call("studioCapture", {
                threadId,
                sessionId,
                frameId: displayed.frameId,
              });
              onReview(capture);
            } catch (e) {
              setError(String((e as Error).message ?? e));
            } finally {
              setSaving(false);
              frozen.current = false;
            }
          }}
        >
          {saving ? "Сохраняю…" : "Снимок и замечание"}
        </Button>
        <span className="text-xs text-muted-foreground">
          960 × 640 · {latency} мс на кадр · OpenGL клиента
        </span>
      </div>
      <p className="grf-notice text-xs text-muted-foreground">
        Живая сцена: камера и воспроизведение. Области и комментарии добавляются
        на сохранённом снимке.
      </p>
      {c && (
        <div className="grf-studio-inspector border-t border-border">
          <strong className="text-sm">Текущий эффект · только чтение</strong>
          <p>
            Попаданий: {c.hitCount} · снарядов: {c.projectileCount} · частиц
            всего: {c.particleCount} · видно: {c.visibleQuads}
          </p>
          <p>
            Полуразмер: {c.definition.halfSize} · подъём: {c.definition.rise} ·
            разброс: {c.definition.spread} world units
          </p>
          <p>Вспышки: {c.impactTicks.map((t) => `${t} tick`).join(", ")}</p>
          <details>
            <summary>Определение, ресурсы и воспроизводимость</summary>
            <p>{c.definitionSource}</p>
            <p>
              Код: {c.rendererVersion.slice(0, 16)} · данные:{" "}
              {c.definitionDigest.slice(0, 16)}
            </p>
            {c.dependencies.map((d) => (
              <p className="break-all" key={d.path}>
                {d.path} · {d.source}
              </p>
            ))}
          </details>
        </div>
      )}
    </div>
  );
}
