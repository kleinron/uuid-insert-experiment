# Published experiment runs

The harness writes ephemeral measure JSON and related files to `results/` at the repo root. Those local outputs stay gitignored so day-to-day reruns do not pollute git.

Committed historical snapshots live here, under `results/runs/<run-id>/`.

## Layout

| Path | Tracked? | Purpose |
| --- | --- | --- |
| `results/` | no (except `.gitkeep`) | Local harness output: `{arm}-{timestamp}.json`, `{arm}-latest.json`, ad-hoc dumps |
| `results/runs/<run-id>/` | yes | Published snapshot of a completed experiment |

`<run-id>` is a directory name such as `2026-09-07-eu-central-1` (date plus region or other distinguishing note).

## Adding a future run

1. Finish the experiment and collect the files you want to keep (measure JSON, CloudWatch exports, InnoDB status, analysis notes, and so on).
2. Copy them into a new directory: `results/runs/<run-id>/`.
3. Include a `README.md` in that directory describing when, where, and how the run was taken.
4. Confirm git will track the snapshot:

   ```bash
   git check-ignore -v results/runs/<run-id>/<some-file>
   ```

   That command should print nothing (the file is not ignored).
5. Commit the new directory. Do not copy ephemeral local files from `results/` into git except via this `runs/<run-id>/` path.
