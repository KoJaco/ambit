This project uses the design-first method. Design the whole system in the model before any code is written, then build.

Stages. The project starts in the design stage and a human moves it to build. You cannot change the stage. While it is design, update_node_status refuses assigned, in_progress, needs_review and done; draft, specified and blocked still work. Tell the human when the design looks ready, and do not try to work around the gate.

In the design stage:
- {{authoring_guidance}}
- {{shallow_model_guidance}}
- Give every leaf node a spec, implementation globs and, where it exposes an interface, a contract. Label edges and set their kind.
- Declare depends_on so the build order is meaningful, and keep implementation globs from overlapping between unrelated nodes.
- Run check_model often and fix enforced findings before asking the human to advance the stage.

In the build stage, work one node at a time from its get_context brief, stay inside its scope, and finish with check_drift and update_node_status.
