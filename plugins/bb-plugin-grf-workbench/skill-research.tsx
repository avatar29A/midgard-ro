import { useEffect, useState } from "react";
import {
  experimental_NewThreadComposer as NewThreadComposer,
  useBbNavigate,
  useRpc,
} from "@get-bb/plugin-sdk/app";
import type { rpcContract } from "./contract";
import type { SkillCatalogData } from "./skill-catalog-contract";
import { Button } from "./components/ui/button";
type Skill = SkillCatalogData["skills"][number];
export function researchPrompt(skill: Skill) {
  return `Реализуй визуальный сценарий навыка ${skill.name} (skill ID ${skill.id}) в Skill Studio проекта midgard-ro. Начни с исследования референсов и текущего клиентского кода.\n\nEF-привязки: ${skill.bindings.map((b) => `${b.phase}: ${b.effects.join(", ")}`).join("; ") || "не найдены в таблице"}.\nИсточники: ${skill.sources.join(", ")}.\nИспользуй общий игровой renderer и учитывай цель (${skill.target || "требует уточнения"}), каст, завершение, время и ресурсы. Каталог и исходники текущего checkout — через grf_skill_catalog и grf_effect_library. Сначала установи, какие части уже реализованы; EF-привязка не доказывает готовность preview.`;
}
export function SkillResearch({
  threadId,
  skill,
  onClose,
}: {
  threadId: string;
  skill: Skill;
  onClose: () => void;
}) {
  const rpc = useRpc<typeof rpcContract>(),
    navigate = useBbNavigate();
  const [operationId] = useState(() => crypto.randomUUID());
  const [context, setContext] = useState<{
      projectId: string;
      environmentId: string;
    } | null>(null),
    [error, setError] = useState("");
  useEffect(() => {
    let live = true;
    rpc.call("researchContext", { threadId }).then(
      (c) => {
        if (live) setContext(c);
      },
      (e) => {
        if (live) setError(String(e.message ?? e));
      },
    );
    return () => {
      live = false;
    };
  }, [rpc, threadId]);
  return (
    <section
      className="grf-skill-summary border-b border-border"
      aria-label={`Исследование ${skill.name}`}
    >
      <div className="grf-toolbar">
        <strong>
          Новый тред · {skill.name} · #{skill.id}
        </strong>
        <Button size="sm" variant="ghost" onClick={onClose}>
          Закрыть подготовку
        </Button>
      </div>
      {error && (
        <p role="alert" className="text-destructive">
          {error}
        </p>
      )}
      {context ? (
        <NewThreadComposer
          key={`${threadId}:${skill.id}`}
          draftKey={`grf-skill-research:${threadId}:${skill.id}`}
          defaultProjectId={context.projectId}
          defaultEnvironment={{
            type: "reuse",
            environmentId: context.environmentId,
          }}
          initialPrompt={researchPrompt(skill)}
          layout="document"
          onSubmit={async (request) => {
            setError("");
            let result;
            try {
              result = await rpc.call("startResearch", {
                operationId,
                sourceThreadId: threadId,
                skillId: skill.id,
                skillName: skill.name,
                request,
              });
            } catch (e) {
              setError(String((e as Error).message ?? e));
              throw e;
            }
            navigate.toThread(result.threadId);
            onClose();
          }}
        />
      ) : (
        !error && <p>Загружаю настройки нового треда…</p>
      )}
    </section>
  );
}
