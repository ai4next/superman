Process this completed session and update only worthwhile long-lived assets.

Evolution scope:
- Role: {{.Role}}
- Agent: {{.AgentName}}
- Root: {{.RootDir}}
- Facts: {{.L1Path}}
- SOPs: {{.SOPDir}}/*.md
{{if .SoulPath}}- Agent soul: {{.SoulPath}}
{{end}}{{if .ExpertDir}}- Experts: {{.ExpertDir}}/{name}/soul.md
{{end}}
Session log: {{.SessionLogPath}}

Current write limits:
{{if .CanAddL1Section}}- You may create a new fact section when no existing section fits.
{{else}}- Do not create new fact sections; update existing sections or skip.
{{end}}{{if .CanDeleteL1Section}}- You may merge or delete weak fact sections before adding stronger facts.
{{end}}{{if .MetaEvolution}}- This is meta evolution. Update only the evolver's own memory under the listed Facts and SOPs paths.
- Do not edit Superman memory, expert memory, expert souls, session logs, prompt templates, source code, or expert team membership.
- Do not trigger or request another meta-evolution pass.
{{end}}{{if .CanEditSoul}}- You may patch this expert's own soul only when the completed session reveals a durable improvement to its role or operating rules.
- Do not create, delete, rename, merge, or otherwise govern other experts.
{{end}}{{if .CultivateExperts}}- Autonomously govern the expert team when the completed Superman session reveals a durable routing or specialization need; no user instruction is required.
{{if .CanCreateExpert}}- You may create a new expert soul for a clear recurring, focused task pattern.
{{else}}- Do not create new expert soul files because the configured expert limit has been reached; improve, merge, retire, or leave existing experts instead.
{{end}}{{end}}
