import { StudioSoundPlayer } from "./studio-audio";
import { memo, useEffect, useRef, useState } from "react";
import { useBbNavigate, useRpc } from "@get-bb/plugin-sdk/app";
import type { Capture, rpcContract } from "./contract";
import {
  defaultStudioScene,
  sceneForSkill,
  hasTargetHits,
  hasGround,
  isPersistent,
  type PreviewSkillId,
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
      alt={`${frame.context.skillName ?? "Soul Strike"} · tick ${frame.context.tick} · ${frame.context.scene.camera.yaw}°`}
      draggable={false}
    />
  ) : (
    <div className="grf-studio-placeholder">Загрузка сцены Mage → Rocker</div>
  );
});

export function SkillStudio({
  threadId,
  skillId = 13,
  skillName = "Soul Strike",
  initialScene,
  onReview,
}: {
  threadId: string | null;
  skillId?: PreviewSkillId;
  skillName?: string;
  initialScene?: StudioScene;
  onReview: (c: Capture) => void;
}) {
  const sound = useRef<StudioSoundPlayer | null>(null);
  sound.current ??= new StudioSoundPlayer();
  const [soundOn, setSoundOn] = useState(false),
    [soundBusy, setSoundBusy] = useState(false),
    [soundError, setSoundError] = useState("");
  const soundEnabled = useRef(false),
    soundGeneration = useRef(0);
  useEffect(
    () => () => {
      soundGeneration.current++;
      void sound.current?.dispose();
    },
    [],
  );
  const rpc = useRpc<typeof rpcContract>(),
    navigate = useBbNavigate();
  const [scene, setScene] = useState<StudioScene>(
    initialScene ?? sceneForSkill(skillId),
  );
  const [sessionId, setSessionId] = useState(() => crypto.randomUUID());
  const [frame, setFrame] = useState<StudioFrame | null>(null),
    [playing, setPlaying] = useState(false),
    [speed, setSpeed] = useState(1);
  const [error, setError] = useState(""),
    [loading, setLoading] = useState(true),
    [saving, setSaving] = useState(false),
    [latency, setLatency] = useState(0);
  const [mode, setMode] = useState<"orbit" | "pan">(
      initialScene?.strResourceId ? "pan" : "orbit",
    ),
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
    if (
      next.tick !== current.current.tick ||
      next.level !== current.current.level ||
      next.hits !== current.current.hits ||
      next.sequence !== current.current.sequence
    ) {
      sound.current?.reset();
      if (play.current && soundEnabled.current) sound.current?.begin(next.tick);
    }
    current.current = next;
    serial.current++;
    setScene(next);
    wake.current();
  }
  function togglePlay(value: boolean) {
    if (play.current !== value) {
      resetClock.current();
      if (value && soundEnabled.current) {
        void sound.current?.unlock().catch((e) => setSoundError(String(e)));
        sound.current?.begin(current.current.tick);
      } else sound.current?.stop();
    }
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
            if (play.current && soundEnabled.current) {
              try {
                sound.current?.advance(result, rate.current);
              } catch (e) {
                setSoundError(String(e));
                soundEnabled.current = false;
                setSoundOn(false);
              }
            }
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
  const rawSTR = !!scene.strResourceId;
  const hitsInput = !rawSTR && hasTargetHits(skillId);
  const statusInput = skillId === 15 || skillId === 16;
  const lifetimeInput = isPersistent(skillId) || statusInput;
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
        <strong>{skillName}</strong>
        <span className="text-xs text-muted-foreground">
          {rawSTR ? "STR · 2D · рендер клиента" : "Маг · общий рендер клиента"}
        </span>
        {!scene.libraryEntryId && !rawSTR && (
          <Button
            size="sm"
            variant="ghost"
            onClick={() =>
              navigate.toPluginPanel("review", {
                subPath: `studio/${threadId}`,
              })
            }
          >
            Развернуть
          </Button>
        )}
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
        {rawSTR
          ? "Исходный STR: слои и смешивание из клиента. Колесо — масштаб, Space + drag — положение. Отдельная 2D-анимация без акторов и остальных частей навыка."
          : skillId === 13
            ? "Каст → выпуск → полёт → попадания → завершение. Поза Mage при касте пока idle, как в клиенте."
            : skillId === 10
              ? "Sight следует за кастером и снимается событием статуса. Время жизни задаёт тестовая сцена."
              : skillId === 16
                ? "Stone Curse: доступны каст и вспышка на цели. Клиент хранит состояние окаменения, но пока не рисует окаменевшее тело. На шкале можно проверить момент его применения и снятия."
                : skillId === 21
                  ? "Thunder Storm: наземная анимация клиента. Отдельные серверные пакеты урона и повторные эффекты от них здесь не моделируются."
                  : !isPersistent(skillId)
                    ? "Каст и эффекты на Rocker используют код и ресурсы клиента. Число попаданий — вход сцены; один пакет вызывает одну реакцию Hurt."
                    : "Эффект закреплён на земле и существует до удаления тестовых skill units. Боевые столкновения не рассчитываются."}
      </p>
      {!rawSTR && (
        <>
          <div className="grf-toolbar">
            <label>
              Просмотр{" "}
              <select
                aria-label="Последовательность навыка"
                value={scene.sequence ? "full" : "volley"}
                onChange={(e) => {
                  togglePlay(false);
                  const next = { ...current.current, tick: 0 };
                  if (e.target.value === "full")
                    next.sequence = {
                      castMs: -1,
                      cancelTick: null,
                      reaction: hasTargetHits(skillId),
                    };
                  else delete next.sequence;
                  change(next);
                }}
              >
                <option value="full">Полный навык</option>
                <option value="volley">
                  {skillId === 13 ? "Только залп" : "Только эффект"}
                </option>
              </select>
            </label>
            {scene.sequence && (
              <>
                <label>
                  Каст, мс{" "}
                  <input
                    aria-label="Длительность каста"
                    type="number"
                    min={0}
                    max={6000}
                    step={50}
                    value={
                      scene.sequence.castMs < 0
                        ? (c?.timeline?.castMs ?? 500)
                        : scene.sequence.castMs
                    }
                    onChange={(e) => {
                      const n = Number(e.target.value);
                      if (!Number.isInteger(n) || n < 0 || n > 6000) return;
                      togglePlay(false);
                      change({
                        ...current.current,
                        tick: 0,
                        sequence: {
                          ...current.current.sequence!,
                          castMs: n,
                          cancelTick: null,
                        },
                      });
                    }}
                  />
                </label>
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={() => {
                    togglePlay(false);
                    change({
                      ...current.current,
                      tick: 0,
                      sequence: {
                        ...current.current.sequence!,
                        castMs: -1,
                        cancelTick: null,
                      },
                    });
                  }}
                >
                  Из таблицы
                </Button>
                <label>
                  Сценарий{" "}
                  <select
                    aria-label="Исход каста"
                    value={
                      scene.sequence.cancelTick === null ? "complete" : "cancel"
                    }
                    onChange={(e) => {
                      togglePlay(false);
                      change({
                        ...current.current,
                        tick: 0,
                        sequence: {
                          ...current.current.sequence!,
                          cancelTick:
                            e.target.value === "cancel"
                              ? Math.floor(
                                  ((current.current.sequence!.castMs < 0
                                    ? (c?.timeline?.castMs ?? 500)
                                    : current.current.sequence!.castMs) *
                                    60) /
                                    2000,
                                )
                              : null,
                        },
                      });
                    }}
                  >
                    <option value="complete">Успешный каст</option>
                    <option
                      value="cancel"
                      disabled={
                        !c?.timeline ||
                        c.timeline.castMs === 0 ||
                        c.scene.level !== scene.level ||
                        c.scene.sequence?.castMs !== scene.sequence.castMs
                      }
                    >
                      Отмена посередине
                    </option>
                  </select>
                </label>
                {hitsInput && (
                  <>
                    <label>
                      <input
                        type="checkbox"
                        aria-label="Реакция цели"
                        checked={scene.sequence.reaction}
                        onChange={(e) =>
                          change({
                            ...current.current,
                            sequence: {
                              ...current.current.sequence!,
                              reaction: e.target.checked,
                            },
                          })
                        }
                      />
                      Реакция цели
                    </label>
                  </>
                )}
              </>
            )}
            <label>
              <input
                type="checkbox"
                aria-label="Звук"
                checked={soundOn}
                disabled={soundBusy || !scene.sequence}
                onChange={async (e) => {
                  const enable = e.target.checked,
                    revision = ++soundGeneration.current;
                  togglePlay(false);
                  soundEnabled.current = false;
                  setSoundOn(false);
                  sound.current?.stop();
                  if (!enable) return;
                  setSoundBusy(true);
                  setSoundError("");
                  try {
                    await sound.current!.unlock();
                    const audio = await rpc.call("studioAudio", {
                      threadId,
                      skillId,
                    });
                    if (revision !== soundGeneration.current) return;
                    await sound.current!.load(audio.clips);
                    if (revision !== soundGeneration.current) return;
                    soundEnabled.current = true;
                    setSoundOn(true);
                    setSoundError(audio.warnings.join("\n"));
                  } catch (e) {
                    if (revision === soundGeneration.current)
                      setSoundError(String((e as Error).message ?? e));
                  } finally {
                    if (revision === soundGeneration.current)
                      setSoundBusy(false);
                  }
                }}
              />
              {soundBusy ? "Загружаю звук…" : "Звук"}
            </label>
          </div>
          {soundError && (
            <p
              role="status"
              className="grf-notice text-xs text-muted-foreground"
            >
              Звук: {soundError}
            </p>
          )}
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
            {hitsInput && (
              <>
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
              </>
            )}
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
          {statusInput && (
            <div className="grf-toolbar">
              <label>
                Состояние цели (вход){" "}
                <select
                  aria-label="Состояние цели"
                  value={scene.targetStatus ?? "none"}
                  onChange={(e) => {
                    togglePlay(false);
                    change({
                      ...current.current,
                      tick: 0,
                      targetStatus: e.target
                        .value as StudioScene["targetStatus"],
                    });
                  }}
                >
                  <option value="none">Без состояния</option>
                  {skillId === 15 ? (
                    <option value="frozen">Заморозка</option>
                  ) : (
                    <option value="stone">Окаменение · пока без визуала</option>
                  )}
                </select>
              </label>
              <label>
                После выпуска, мс{" "}
                <input
                  aria-label="Задержка состояния"
                  type="number"
                  min={0}
                  max={10000}
                  step={50}
                  value={scene.statusDelayMs ?? 500}
                  onChange={(e) => {
                    const n = Number(e.target.value);
                    if (Number.isInteger(n) && n >= 0 && n <= 10000) {
                      togglePlay(false);
                      change({ ...current.current, tick: 0, statusDelayMs: n });
                    }
                  }}
                />
              </label>
            </div>
          )}
          {(lifetimeInput || hasGround(skillId)) && (
            <div className="grf-toolbar">
              {lifetimeInput && (
                <label>
                  {statusInput ? "Состояние, мс" : "Эффект, мс"}{" "}
                  <input
                    aria-label="Время жизни эффекта"
                    type="number"
                    min={100}
                    max={30000}
                    step={100}
                    value={scene.effectDurationMs ?? 3000}
                    onChange={(e) => {
                      const n = Number(e.target.value);
                      if (Number.isInteger(n) && n >= 100 && n <= 30000) {
                        togglePlay(false);
                        change({
                          ...current.current,
                          tick: 0,
                          effectDurationMs: n,
                        });
                      }
                    }}
                  />
                </label>
              )}
              {hasGround(skillId) && (
                <>
                  <label>
                    Разместить{" "}
                    <select
                      aria-label="Положение эффекта"
                      value={scene.ground?.anchor ?? "target"}
                      onChange={(e) =>
                        change({
                          ...current.current,
                          ground: {
                            anchor: e.target.value as "caster" | "target",
                            cells:
                              current.current.ground?.cells ??
                              (skillId === 18 ? 3 : 1),
                            angle: current.current.ground?.angle ?? 0,
                          },
                        })
                      }
                    >
                      <option value="target">У Rocker</option>
                      <option value="caster">У Mage</option>
                    </select>
                  </label>
                  {skillId === 18 && (
                    <>
                      <label>
                        Клеток{" "}
                        <select
                          aria-label="Клетки Fire Wall"
                          value={scene.ground?.cells ?? 3}
                          onChange={(e) =>
                            change({
                              ...current.current,
                              ground: {
                                anchor:
                                  current.current.ground?.anchor ?? "target",
                                cells: Number(e.target.value) as 1 | 3,
                                angle: current.current.ground?.angle ?? 0,
                              },
                            })
                          }
                        >
                          <option value={1}>1</option>
                          <option value={3}>3</option>
                        </select>
                      </label>
                      <label>
                        Линия{" "}
                        <select
                          aria-label="Направление Fire Wall"
                          value={scene.ground?.angle ?? 0}
                          onChange={(e) =>
                            change({
                              ...current.current,
                              ground: {
                                anchor:
                                  current.current.ground?.anchor ?? "target",
                                cells: current.current.ground?.cells ?? 3,
                                angle: Number(e.target.value) as 0 | 90,
                              },
                            })
                          }
                        >
                          <option value={0}>Вдоль Z</option>
                          <option value={90}>Вдоль X</option>
                        </select>
                      </label>
                    </>
                  )}
                </>
              )}
            </div>
          )}
        </>
      )}
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
          aria-label={`Время ${skillName}`}
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
          onChange={(e) => {
            const n = Number(e.target.value);
            setSpeed(n);
            sound.current?.setRate(n);
          }}
        >
          {[0.25, 0.5, 1, 2].map((n) => (
            <option key={n} value={n}>
              {n}×
            </option>
          ))}
        </select>
      </div>
      {c?.timeline && (
        <div className="grf-studio-timeline border-y border-border">
          <p className="text-xs">
            Фаза:{" "}
            {
              {
                cast: "Каст",
                volley: "Полёт",
                tail: "Остаточный след",
                finished: "Завершено",
                canceled: "Каст отменён",
                active: "Эффект активен",
              }[c.phase]
            }
          </p>
          {[
            [
              "Каст",
              [
                "cast.started",
                "cast.released",
                "cast.canceled",
                "effect.finished",
              ],
            ],
            ["Попадания", ["visual.impact"]],
            ["Реакция", ["target.hurt"]],
            [
              "Эффект",
              [
                "effect.started",
                "status.applied",
                "status.removed",
                "unit.created",
                "unit.removed",
              ],
            ],
            ["Звук", ["sound"]],
          ].map(([label, kinds]) => (
            <div className="grf-studio-track" key={label as string}>
              <span className="text-xs text-muted-foreground">
                {label as string}
              </span>
              {c
                .timeline!.events.filter((e) =>
                  (kinds as string[]).includes(e.kind),
                )
                .map((e) => (
                  <Button
                    key={e.id}
                    size="sm"
                    variant={scene.tick === e.tick ? "secondary" : "ghost"}
                    onClick={() => {
                      togglePlay(false);
                      change({ ...current.current, tick: e.tick });
                    }}
                  >
                    {e.kind === "effect.started"
                      ? "Эффект"
                      : e.kind === "cast.started"
                        ? "Начало"
                        : e.kind === "cast.released"
                          ? "Выпуск"
                          : e.kind === "cast.canceled"
                            ? "Отмена"
                            : e.kind === "effect.finished"
                              ? "Конец"
                              : e.kind === "status.applied"
                                ? "Включение"
                                : e.kind === "status.removed"
                                  ? "Снятие"
                                  : e.kind === "unit.created"
                                    ? "Создание"
                                    : e.kind === "unit.removed"
                                      ? "Удаление"
                                      : e.kind === "target.hurt"
                                        ? "Hurt"
                                        : e.kind === "sound"
                                          ? e.owner === "caster"
                                            ? "Каст"
                                            : "Удар"
                                          : "Попадание"}{" "}
                    · {e.tick}
                  </Button>
                ))}
            </div>
          ))}
        </div>
      )}
      <div className="grf-toolbar">
        {!rawSTR && (
          <Button
            size="sm"
            variant={mode === "orbit" ? "secondary" : "ghost"}
            aria-pressed={mode === "orbit"}
            aria-description="Средняя кнопка мыши или перетаскивание в режиме вращения"
            onClick={() => setMode("orbit")}
          >
            <CameraModeIcon mode="orbit" /> Вращать
          </Button>
        )}
        <Button
          size="sm"
          variant={mode === "pan" ? "secondary" : "ghost"}
          aria-pressed={mode === "pan"}
          aria-description="Space + левая кнопка или Shift + средняя кнопка"
          onClick={() => setMode("pan")}
        >
          <CameraModeIcon mode="pan" /> Перемещать
        </Button>
        {!rawSTR && (
          <>
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
          </>
        )}
        <Button
          size="sm"
          variant="ghost"
          onClick={() => cameraChange(defaultStudioScene.camera)}
        >
          {rawSTR ? "Масштаб 1:1 и центр" : "Игровой ракурс"}
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
      {c?.limitations
        .filter((message) => message.includes("skipped as in the client"))
        .map((message) => (
          <p key={message} role="status" className="grf-notice">
            Ресурс недоступен: {message}
          </p>
        ))}
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
            mode:
              rawSTR || space || e.shiftKey
                ? "pan"
                : e.button === 1
                  ? "orbit"
                  : mode,
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
          {rawSTR && (
            <p>
              Кадр STR: {Math.floor(Number(c.sourceFrame ?? 0))} ·{" "}
              {Math.round(c.timeMs)} мс
            </p>
          )}
          {skillId === 13 && !rawSTR ? (
            <>
              <p>
                Попаданий: {c.hitCount} · снарядов: {c.projectileCount} · частиц
                всего: {c.particleCount} · видно: {c.visibleQuads}
              </p>
              <p>
                Полуразмер: {c.definition.halfSize} · подъём:{" "}
                {c.definition.rise} · разброс: {c.definition.spread} world units
              </p>
              <p>Вспышки: {c.impactTicks.map((t) => `${t} tick`).join(", ")}</p>
            </>
          ) : (
            <>
              <p>Видимых элементов: {c.visibleQuads}</p>
              {hitsInput && (
                <p>
                  Входных попаданий: {c.hitCount} · снарядов:{" "}
                  {c.projectileCount} · меток попаданий: {c.impactTicks.length}
                </p>
              )}
              {Object.entries(c.definition.parameters ?? {}).map(
                ([key, value]) => (
                  <p key={key}>
                    {key}: {String(value)}
                  </p>
                ),
              )}
            </>
          )}
          <details>
            <summary>Определение, ресурсы и воспроизводимость</summary>
            {c.limitations.map((message) => (
              <p key={message}>{message}</p>
            ))}
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
