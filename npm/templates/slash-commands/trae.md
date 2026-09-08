# Trae `/centmem` Slash Command Adapter

This adapter provides smart routing for the `/centmem` slash command in Trae.

## Routing Prompt

<!-- centmem-command:start -->
## /centmem Command
When the user types `/centmem <input>`, act as the Memory Manager. Analyze the intent:
- **Recall / Context**: If asking a question or looking for context, run `centmem recall "<input>" --scope "project:$CENTMEM_PROJ" --top 5` or `centmem timeline`.
- **Save Decisions / Facts**: If stating a decision, convention, preference, or learning to save, run `centmem put --scope "project:$CENTMEM_PROJ" --type note --content "<input>"` or `centmem set --scope "project:$CENTMEM_PROJ" --key "<key>" --value '<json>'`.
- **Relationships / Links**: If asking to link memories, inspect relationships, trace dependencies/contradictions, or confirm/dismiss auto-suggestions, run centmem link, centmem links, or centmem unlink.
- **Ingest / Capture**: If asking to ingest repository knowledge, commits, docs, shell history, or code annotations, run `centmem capture git`, `centmem capture docs`, `centmem capture shell`, or `centmem capture comments`.
- **Maintenance / Health**: If requesting maintenance, health checks, or statistics, run `centmem doctor`, `centmem stats`, or `centmem compact`.
- **Visual Dashboard / UI**: If asking to view, browse, inspect, or manage memories in a browser interface, run `centmem ui`.

Always verify execution results from stdout JSON and report them clearly to the user.
<!-- centmem-command:end -->
