# Background proposals and call inspector

Interactive AI setup starts an independent worker after the context-send
confirmation. The inspector follows that worker. Leaving the inspector does
not cancel the request and does not write project files.

```text
harnessforge init --repository <project>
harnessforge init --repository <project> --background
harnessforge init runs
harnessforge init inspect <run-id>
harnessforge init inspect <run-id> --accessible
harnessforge init resume <run-id>
harnessforge init cancel <run-id>
harnessforge init forget <run-id>
```

`--background` returns immediately after starting the worker. Without that
flag, interactive setup opens the inspector. Redirected input/output and
`--accessible` preserve the synchronous plain-text setup unless background
execution is explicitly requested.

The inspector shows actual context-preparation, provider-call and validation
events, elapsed time, HTTP responses and retry attempts. It never estimates
percent completion, shows document bodies, or shows credentials. HTTP status
is available only after response headers are actually received. This does not
mean the response body has finished arriving. Events are ordered by their real
timestamps. Use `--language pt-BR` when reopening the inspector in Portuguese.

Generation and applying are separate operations. `init resume` re-reads the
selected files, checks they match the authorized context, verifies proposal
citations, previews the generated files, and requires explicit confirmation.
Provider failures retain the option of a local proposal.

Run state is stored outside the project, in the user's cache. Credentials and
full context documents are passed to the worker through a pipe, not command
arguments or run-state files. The saved proposal can contain cited excerpts;
the explicit proposal review displays these excerpts. Free-form observations
are not retained as raw notes for subsequent file generation.

Detaching requires keeping this CLI executable available while the child
starts. Background work still depends on the computer staying on and network
availability; this is not a remote job service.

Workers have a four-minute maximum runtime. An interrupted worker with no
final state is treated as failed after six minutes, allowing local recovery
or deletion with `init forget`. The current provider transport also has its
own shorter request timeout. Windows uses inherited per-user cache access
controls; proposal storage is not encrypted by the CLI.
