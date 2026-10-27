# Third-party pins

Every piece of third-party code in this repository, the exact upstream it came from, and its license.
A new entry needs the tag AND the commit sha (a tag can be moved; a sha cannot), and the license file
under third_party/<name>/.

| Name | Upstream | Tag | Commit | License | What was taken, where it lives |
|---|---|---|---|---|---|
| hcsshim | https://github.com/microsoft/hcsshim | v0.15.0 | 900183417d29765de2947de22a6e72c22c195550 | MIT (third_party/hcsshim/LICENSE) | Job Object wrapper from internal/jobobject + internal/winapi -> internal/procjob/job_windows.go; pseudoconsole wrapper from internal/conpty -> internal/llm/tools/shell/conpty_windows.go. Copied, not imported. No logging, OpenTelemetry or ETW code was taken. |
