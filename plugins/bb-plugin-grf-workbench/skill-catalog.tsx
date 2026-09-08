import { SkillResearch } from "./skill-research";
import type { PreviewSkillId } from "./studio-contract";
import { useEffect, useMemo, useState } from "react";
import { useRpc } from "@get-bb/plugin-sdk/app";
import type { Capture, rpcContract } from "./contract";
import type { StudioScene } from "./studio-contract";
import type { SkillCatalogData } from "./skill-catalog-contract";
import { SkillStudio } from "./skill-studio";
import { Button } from "./components/ui/button";
import { Input } from "./components/ui/input";
const status = {
  partial: "Частичный просмотр · пробел в клиенте",
  preview: "Есть просмотр в Studio",
  mapped: "Есть EF-привязки",
  unmapped: "Нет EF-привязок",
};
const phase = {
  cast: "При касте",
  caster: "На кастере",
  target: "На цели",
  ground: "На земле",
};
export function SkillCatalog({
  threadId,
  initialScene,
  initialSkillId,
  onLibrary,
  onReview,
}: {
  threadId: string | null;
  initialScene?: StudioScene;
  initialSkillId?: number;
  onLibrary?: (skillId: number, query?: string) => void;
  onReview: (c: Capture) => void;
}) {
  const rpc = useRpc<typeof rpcContract>();
  const [research, setResearch] = useState(false);
  const [data, setData] = useState<SkillCatalogData | null>(null),
    [error, setError] = useState("");
  const [job, setJob] = useState(initialSkillId ? -1 : 2),
    [showUnknownJobs, setShowUnknownJobs] = useState(false),
    [query, setQuery] = useState(""),
    [filter, setFilter] = useState("all"),
    [selected, setSelected] = useState<number | null>(
      initialSkillId ?? initialScene?.skillId ?? 13,
    ),
    [refresh, setRefresh] = useState(0);
  useEffect(() => {
    if (!threadId) return;
    let live = true;
    setError("");
    rpc.call("skillCatalog", { threadId }).then(
      (c) => {
        if (live) setData(c);
      },
      (e) => {
        if (live) setError(String(e.message ?? e));
      },
    );
    return () => {
      live = false;
    };
  }, [threadId, rpc, refresh]);
  const visibleJobs = useMemo(
    () =>
      data?.jobs.filter(
        (j) => showUnknownJobs || !/^unknown(?:\s|\(|$)/i.test(j.name),
      ) ?? [],
    [data, showUnknownJobs],
  );
  useEffect(() => {
    if (data && job !== -1 && !visibleJobs.some((j) => j.id === job))
      setJob(
        visibleJobs.find((j) => j.id === 2)?.id ?? visibleJobs[0]?.id ?? -1,
      );
  }, [data, job, visibleJobs]);
  const rows = useMemo(() => {
    if (!data) return [];
    const q = query.trim().toLowerCase();
    const own = data.jobs.find((j) => j.id === job)?.skills;
    const found = data.skills.filter(
      (s) =>
        (job === -1 || s.jobs.includes(job)) &&
        (filter === "all" || s.studio === filter) &&
        `${s.name} ${s.id} ${s.icon} ${s.bindings.flatMap((b) => b.effects).join(" ")}`
          .toLowerCase()
          .includes(q),
    );
    return job === -1
      ? found
      : found.sort(
          (a, b) => (own?.indexOf(a.id) ?? 0) - (own?.indexOf(b.id) ?? 0),
        );
  }, [data, job, query, filter]);
  useEffect(() => {
    if (data && !rows.some((s) => s.id === selected))
      setSelected(rows[0]?.id ?? null);
  }, [data, rows, selected]);
  const skill = data?.skills.find((s) => s.id === selected);
  useEffect(() => setResearch(false), [selected]);
  if (!threadId)
    return <p className="grf-notice">Откройте Studio из обсуждения проекта.</p>;
  return (
    <div className="grf-skill-catalog">
      <div className="grf-toolbar border-b border-border">
        <label className="text-xs">
          Профессия{" "}
          <select
            aria-label="Профессия"
            value={job}
            onChange={(e) => setJob(Number(e.target.value))}
          >
            <option value={-1}>Все навыки клиента</option>
            {data ? (
              visibleJobs.map((j) => (
                <option key={j.id} value={j.id}>
                  {j.name} · {j.skills.length}
                </option>
              ))
            ) : (
              <option value={2}>Mage</option>
            )}
          </select>
        </label>
        <label className="text-xs">
          <input
            type="checkbox"
            checked={showUnknownJobs}
            onChange={(e) => setShowUnknownJobs(e.target.checked)}
          />{" "}
          Показать Unknown
        </label>
        <Input
          aria-label="Поиск навыка"
          placeholder="Название, ID или эффект…"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
        />
        <select
          aria-label="Статус навыка"
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
        >
          <option value="all">Все статусы</option>
          {Object.entries(status).map(([v, name]) => (
            <option key={v} value={v}>
              {name}
            </option>
          ))}
        </select>
        <Button
          size="sm"
          variant="ghost"
          onClick={() => setRefresh((n) => n + 1)}
        >
          Обновить каталог
        </Button>
      </div>
      {error && (
        <p role="alert" className="grf-notice text-destructive">
          Каталог: {error}
        </p>
      )}
      {!data && !error && (
        <p role="status" className="grf-notice text-xs">
          Читаю таблицы клиента…
        </p>
      )}
      <div className="grf-catalog-compact">
        <select
          aria-label="Навык"
          value={selected ?? ""}
          onChange={(e) => setSelected(Number(e.target.value))}
        >
          {!rows.length && <option value="">Нет совпадений</option>}
          {rows.map((s) => (
            <option key={s.id} value={s.id}>
              {s.name} · {s.id} · {status[s.studio]}
            </option>
          ))}
        </select>
      </div>
      <div className="grf-catalog-body">
        <aside className="grf-catalog-list border-r border-border">
          <p className="text-xs text-muted-foreground">
            {rows.length} навыков · страницы как в клиенте
          </p>
          {rows.map((s) => (
            <button
              type="button"
              aria-pressed={selected === s.id}
              className={`grf-resource-row ${selected === s.id ? "bg-primary/10" : ""}`}
              key={s.id}
              onClick={() => setSelected(s.id)}
            >
              <span>{s.name}</span>
              <span className="text-xs text-muted-foreground">
                #{s.id} · {status[s.studio]}
              </span>
            </button>
          ))}
        </aside>
        <section className="grf-catalog-detail">
          {skill && (
            <div className="grf-skill-summary border-b border-border">
              <strong>{skill.name}</strong>{" "}
              <span className="text-xs text-muted-foreground">
                #{skill.id} · {status[skill.studio]}
              </span>
              {onLibrary && (
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => onLibrary(skill.id)}
                >
                  Эффекты в библиотеке
                </Button>
              )}
              <details open={skill.studio !== "preview"}>
                <summary>Данные навыка и эффекты</summary>
                <p className="text-sm">
                  Тип: {skill.kind || "не указан"} · Цель:{" "}
                  {skill.target || "не указана"}
                </p>
                {skill.bindings.map((b) => (
                  <p className="text-sm" key={b.phase}>
                    {phase[b.phase]}: {b.effects.join(", ")}
                  </p>
                ))}
                {!skill.bindings.length && (
                  <p className="text-sm text-muted-foreground">
                    Отсутствие привязки не означает ошибку или отсутствие
                    визуала: назначение нужно проверить, в том числе для
                    пассивных навыков.
                  </p>
                )}
                {[
                  ["Попадания", skill.hits],
                  ["Каст, мс", skill.castMs],
                  ["Фиксированная часть, мс", skill.fixedMs],
                  ["Элемент", skill.elements],
                ].map(([label, values]) => (
                  <p className="text-xs" key={label as string}>
                    {label as string}:{" "}
                    {(values as (string | number)[]).join(" / ") ||
                      "не указано"}
                  </p>
                ))}
                <p className="text-xs text-muted-foreground">
                  Это значения таблицы по уровням; последнее повторяется на
                  следующих уровнях. Максимальный уровень и пассивность этим
                  экспортом не определяются.
                </p>
                <p className="text-xs">
                  Также на страницах:{" "}
                  {skill.jobs
                    .map(
                      (id) =>
                        data?.jobs.find((j) => j.id === id)?.name ?? String(id),
                    )
                    .join(", ") || "нет страницы профессии"}
                </p>
                <details>
                  <summary>Источники</summary>
                  {skill.sources.map((p) => (
                    <p key={p} className="text-xs break-all">
                      {p}
                    </p>
                  ))}
                  <p className="text-xs">
                    Ревизия данных: {data?.sourceVersion.slice(0, 16)}
                  </p>
                </details>
              </details>
              {skill.studio !== "preview" && (
                <Button size="sm" onClick={() => setResearch(true)}>
                  Подготовить исследование в новом треде
                </Button>
              )}
            </div>
          )}
          {research && skill && (
            <SkillResearch
              key={`${threadId}:${skill.id}`}
              threadId={threadId}
              skill={skill}
              onClose={() => setResearch(false)}
            />
          )}
          {skill?.studio === "preview" ||
          skill?.studio === "partial" ||
          (!data && selected === 13) ? (
            <SkillStudio
              key={selected}
              skillId={(selected ?? 13) as PreviewSkillId}
              skillName={skill?.name ?? "Soul Strike"}
              threadId={threadId}
              initialScene={
                (initialScene?.skillId ?? 13) === selected
                  ? initialScene
                  : undefined
              }
              onReview={onReview}
            />
          ) : (
            <p className="grf-notice text-muted-foreground">
              {skill
                ? "Просмотр этого навыка ещё не подключён к Studio. Карточка показывает данные клиента; подготовьте исследование для подключения этого навыка."
                : data
                  ? "Нет навыков по выбранным условиям."
                  : "Загружаю каталог…"}
            </p>
          )}
        </section>
      </div>
    </div>
  );
}
