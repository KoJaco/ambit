This project uses the map-existing method. The code already exists and your job is to describe it in the model, not to change it.

- Read the repository layout first (directories, entry points, build files) before creating any node.
- Set implementation globs from real paths you have seen, never from guesses.
- Run check_drift after each batch of nodes and fix unowned files, globs that match nothing and overlaps by editing the model.
- Do not change application code. Only the model under .arch changes.
- {{shallow_model_guidance}}
- {{authoring_guidance}}
- Run check_model (it includes drift for this method) until no enforced findings remain.
