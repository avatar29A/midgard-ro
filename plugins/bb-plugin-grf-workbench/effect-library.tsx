import { useEffect, useState } from "react";
import { useRpc } from "@get-bb/plugin-sdk/app";
import type { Capture, rpcContract } from "./contract";
import type {
  EffectLibraryData,
  LibraryEntry,
} from "./effect-library-contract";
import {
  defaultStudioScene,
  sceneForSkill,
  type PreviewSkillId,
  type StudioScene,
} from "./studio-contract";
import { SkillStudio } from "./skill-studio";
import { AssetBrowser } from "./asset-browser";
import { Button } from "./components/ui/button";
import { Input } from "./components/ui/input";
const kindNames: Record<string, string> = {
  procedural: "Процедурный",
  composite: "Составной",
  binding: "EF-привязка",
  str: "STR",
  act: "ACT",
  spr: "SPR",
};
export function EffectLibrary({
  threadId,
  initialScene,
  initialSkill = 0,
  initialQuery = "",
  onSkill,
  onReview,
}: {
  threadId: string | null;
  initialScene?: StudioScene;
  initialSkill?: number;
  initialQuery?: string;
  onSkill: (id: number) => void;
  onReview: (c: Capture) => void;
}) {
  const rpc = useRpc<typeof rpcContract>();
  const [query, setQuery] = useState(initialQuery),
    [mode, setMode] = useState<"all" | "curated" | "bindings" | "resources">(
      initialQuery || initialSkill ? "all" : "curated",
    ),
    [kind, setKind] = useState(""),
    [skill, setSkill] = useState(initialSkill),
    [offset, setOffset] = useState(0),
    [refresh, setRefresh] = useState(0);
  const [data, setData] = useState<EffectLibraryData | null>(null),
    [error, setError] = useState(""),
    [busy, setBusy] = useState(false),
    [selected, setSelected] = useState<LibraryEntry | null>(null),
    [view, setView] = useState<{
      scene?: StudioScene;
      asset?: string;
      name: string;
      skill?: PreviewSkillId;
    } | null>(
      initialScene?.libraryEntryId || initialScene?.strResourceId
        ? {
            scene: initialScene,
            name: initialScene.strResourceId ? "STR" : "Эффект",
            skill: initialScene.skillId,
          }
        : null,
    );
  useEffect(() => {
    if (!threadId) return;
    let live = true;
    setBusy(true);
    setError("");
    const timer = setTimeout(() => {
      rpc
        .call("effectLibrary", {
          threadId,
          query,
          mode,
          kind: kind as
            | ""
            | "procedural"
            | "composite"
            | "binding"
            | "str"
            | "act"
            | "spr",
          skillId: skill,
          offset,
        })
        .then(
          (d) => {
            if (live) {
              setData(d);
              setBusy(false);
            }
          },
          (e) => {
            if (live) {
              setError(String(e.message ?? e));
              setBusy(false);
            }
          },
        );
    }, 200);
    return () => {
      live = false;
      clearTimeout(timer);
    };
  }, [threadId, rpc, query, mode, kind, skill, offset, refresh]);
  function play(
    entry: LibraryEntry,
    resource?: LibraryEntry["resources"][number],
  ) {
    if (resource && resource.type !== "str") {
      setView({ asset: resource.id, name: resource.path });
      return;
    }
    const id = entry.previewSkill as PreviewSkillId;
    const scene: StudioScene = resource
      ? {
          ...defaultStudioScene,
          strResourceId: resource.id,
          sequence: undefined,
          level: 1,
          tick: 0,
        }
      : { ...sceneForSkill(id), sequence: undefined, tick: 0 };
    delete scene.sequence;
    scene.libraryEntryId = entry.id;
    scene.libraryRevision = data?.revision;
    setView({
      scene,
      name: resource ? resource.path : entry.name,
      skill: resource ? undefined : id,
    });
  }
  return (
    <div className="grf-effect-library">
      <div className="grf-toolbar border-b border-border">
        <Input
          aria-label="Поиск эффекта"
          placeholder="Огонь, аура, Soul Strike, EF или путь…"
          value={query}
          onChange={(e) => {
            setQuery(e.target.value);
            setOffset(0);
          }}
        />
        <select
          aria-label="Раздел библиотеки"
          value={mode}
          onChange={(e) => {
            setMode(e.target.value as typeof mode);
            setOffset(0);
          }}
        >
          <option value="curated">Понятные названия</option>
          <option value="bindings">Привязки клиента</option>
          <option value="resources">Все анимации GRF</option>
          <option value="all">Вся библиотека</option>
        </select>
        <select
          aria-label="Тип эффекта"
          value={kind}
          onChange={(e) => {
            setKind(e.target.value);
            setOffset(0);
          }}
        >
          <option value="">Все типы</option>
          {Object.entries(kindNames).map(([id, name]) => (
            <option key={id} value={id}>
              {name}
            </option>
          ))}
        </select>
        {skill !== 0 && (
          <Button
            size="sm"
            variant="ghost"
            onClick={() => {
              setSkill(0);
              setOffset(0);
            }}
          >
            Навык #{skill} · снять фильтр
          </Button>
        )}
        <Button
          size="sm"
          variant="ghost"
          onClick={() => setRefresh((n) => n + 1)}
        >
          Обновить библиотеку
        </Button>
      </div>
      {error && (
        <p role="alert" className="grf-notice text-destructive">
          {error}
        </p>
      )}
      <p className="grf-notice text-xs text-muted-foreground">
        {busy ? "Читаю каталог и архивы…" : `${data?.matched ?? 0} совпадений`}.
        Названия и теги — в проекте. Неразобранные ресурсы сохраняют исходные
        имена.
      </p>
      <div className="grf-catalog-compact">
        <select
          aria-label="Карточка эффекта"
          value={selected?.id ?? ""}
          onChange={(e) => {
            setSelected(
              data?.entries.find((row) => row.id === e.target.value) ?? null,
            );
            setView(null);
          }}
        >
          <option value="">Выберите эффект или анимацию</option>
          {data?.entries.map((e) => (
            <option key={e.id} value={e.id}>
              {e.name} · {kindNames[e.kind]}
            </option>
          ))}
        </select>
        <div className="grf-toolbar">
          <Button
            size="sm"
            disabled={offset === 0 || busy}
            onClick={() => setOffset(Math.max(0, offset - 50))}
          >
            Назад
          </Button>
          <Button
            size="sm"
            disabled={!data || data.next < 0 || busy}
            onClick={() => setOffset(data!.next)}
          >
            Далее
          </Button>
        </div>
      </div>
      <div className="grf-catalog-body">
        <aside className="grf-catalog-list border-r border-border">
          {data?.entries.map((e) => (
            <button
              key={e.id}
              type="button"
              aria-pressed={selected?.id === e.id}
              className={`grf-resource-row ${selected?.id === e.id ? "bg-primary/10" : ""}`}
              onClick={() => {
                setSelected(e);
                setView(null);
              }}
            >
              <span>{e.name}</span>
              <span className="text-xs text-muted-foreground">
                {e.english} · {kindNames[e.kind]} ·{" "}
                {e.curated ? "описан" : "требует исследования"}
              </span>
            </button>
          ))}
          {!busy && data?.matched === 0 && (
            <p>Нет совпадений. Попробуйте другой раздел или снимите фильтры.</p>
          )}
          <div className="grf-toolbar">
            <Button
              size="sm"
              disabled={offset === 0 || busy}
              onClick={() => setOffset(Math.max(0, offset - 50))}
            >
              Назад
            </Button>
            <Button
              size="sm"
              disabled={!data || data.next < 0 || busy}
              onClick={() => setOffset(data!.next)}
            >
              Далее
            </Button>
          </div>
        </aside>
        <section className="grf-catalog-detail">
          {selected && (
            <div className="grf-skill-summary border-b border-border">
              <strong>{selected.name}</strong>
              <p>
                {selected.english} · {kindNames[selected.kind]}
              </p>
              <p className="text-sm">{selected.description}</p>
              <p className="text-xs">{selected.tags.join(" · ")}</p>
              {selected.previewSkill > 0 && (
                <Button size="sm" onClick={() => play(selected)}>
                  Показать эффект в сцене
                </Button>
              )}
              <details open>
                <summary>Исходные ресурсы</summary>
                {selected.resources.map((r, i) => (
                  <div className="grf-toolbar" key={r.path + i}>
                    <span className="text-xs break-all">{r.path}</span>
                    {r.available ? (
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => play(selected, r)}
                      >
                        Открыть {r.type.toUpperCase()}
                      </Button>
                    ) : (
                      <span className="text-xs text-destructive">
                        Не найден в GRF
                      </span>
                    )}
                  </div>
                ))}
                {!selected.resources.length && (
                  <p className="text-xs">
                    Отдельный файл не указан; смотрите зависимости кадра и
                    исходники генератора.
                  </p>
                )}
              </details>
              <details>
                <summary>
                  Используется в навыках (
                  {new Set(selected.skills.map((s) => s.id)).size})
                </summary>
                {selected.skills.map((s) => (
                  <Button
                    key={`${s.id}/${s.phase}`}
                    size="sm"
                    variant="ghost"
                    onClick={() => onSkill(s.id)}
                  >
                    {s.name} · {s.phase}
                  </Button>
                ))}
              </details>
              <details>
                <summary>Идентификатор и источники</summary>
                <p className="text-xs break-all">{selected.id}</p>
                <p className="text-xs">
                  {selected.effects.map((ef) => (
                    <Button
                      key={ef}
                      size="sm"
                      variant="ghost"
                      onClick={() => {
                        setQuery(ef);
                        setMode("bindings");
                        setKind("");
                        setSkill(0);
                        setOffset(0);
                      }}
                    >
                      {ef}
                    </Button>
                  ))}
                </p>
                {selected.sources.map((p) => (
                  <p key={p} className="text-xs break-all">
                    {p}
                  </p>
                ))}
                <p className="text-xs">
                  Каталог: {data?.revision.slice(0, 16)} · код:{" "}
                  {data?.sourceVersion.slice(0, 16)}
                </p>
              </details>
            </div>
          )}
          {view?.scene ? (
            <SkillStudio
              key={JSON.stringify([selected?.id, view.scene])}
              threadId={threadId}
              skillId={view.skill}
              skillName={view.name}
              initialScene={view.scene}
              onReview={onReview}
            />
          ) : view?.asset ? (
            <AssetBrowser
              key={view.asset}
              threadId={threadId}
              initialId={view.asset}
              compact
              onReview={onReview}
            />
          ) : (
            <p className="grf-notice text-muted-foreground">
              Выберите карточку и откройте эффект в сцене или отдельный исходный
              ресурс. SPR — лист изображений; ACT задаёт анимацию; STR может
              быть лишь частью эффекта.
            </p>
          )}
        </section>
      </div>
    </div>
  );
}
