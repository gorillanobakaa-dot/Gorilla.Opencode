# Local copy of cloud.google.com/go/auth v0.13.0 (Apache-2.0, see LICENSE) - changes made for Gorilla OpenCode

Why: the owner's rule (2026-10-18) is that telemetry is cut out of the source, not switched off. This module was the
only importer of OpenTelemetry in the whole Gorilla OpenCode build (pulled in by google.golang.org/genai for the
Gemini provider); with it changed, no go.opentelemetry.io package is linked (`go version -m`, and
`fieldkit telemetry scan --artifact` on the built program).

Changed files (2026-10-18), nothing else:
- httptransport/transport.go: the otelhttp import is removed; addOpenTelemetryTransport returns the transport
  unchanged (what upstream does with Options.DisableTelemetry).
- grpctransport/grpctransport.go: the otelgrpc import, the singleton stats handler and the now unused sync/stats
  imports are removed; addOpenTelemetryStatsHandler returns the dial options unchanged (DisableTelemetry upstream).
- go.mod: the go.opentelemetry.io requirements are removed.

Not copied: upstream's *_test.go files and testdata/ folders. They are not part of Gorilla OpenCode's build, and
testdata/ holds test private keys that the repository's secret check rightly refuses to commit.
Upgrading: copy the new upstream version over this folder and re-apply the three changes above; the release check
fails if OpenTelemetry comes back.
