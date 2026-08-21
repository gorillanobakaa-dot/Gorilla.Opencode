module github.com/opencode-ai/opencode

go 1.24.0

require (
	github.com/JohannesKaufmann/html-to-markdown v1.6.0
	github.com/PuerkitoBio/goquery v1.9.2
	github.com/alecthomas/chroma/v2 v2.15.0
	github.com/anthropics/anthropic-sdk-go v1.4.0
	github.com/aymanbagabas/go-udiff v0.2.0
	github.com/bmatcuk/doublestar/v4 v4.8.1
	github.com/catppuccin/go v0.3.0
	github.com/charmbracelet/bubbles v0.21.0
	github.com/charmbracelet/bubbletea v1.3.5
	github.com/charmbracelet/glamour v0.9.1
	github.com/charmbracelet/lipgloss v1.1.0
	github.com/charmbracelet/x/ansi v0.8.0
	github.com/fsnotify/fsnotify v1.8.0
	github.com/go-logfmt/logfmt v0.6.0
	github.com/google/uuid v1.6.0
	github.com/lrstanley/bubblezone v0.0.0-20250315020633-c249a3fe1231
	// GORILLA OVERRIDE (2026-08-19): this pin is a DECISION, not a default,
	// and it has a cost worth stating.
	//
	// v0.17.0 hard-codes LATEST_PROTOCOL_VERSION = "2024-11-05". Upstream is
	// at v0.58.0 (plus a v1.0.0 beta) and speaks later revisions. So this
	// client asks every MCP server for a two-year-old protocol, and a server
	// that only offers a newer one will negotiate down or fail — which,
	// before 2026-08-19, was invisible: nothing logged the negotiated
	// version, so a capability missing because of a version mismatch looked
	// exactly like a capability the server does not have. getTools now logs
	// what was requested and what was agreed, and warns when they differ.
	//
	// Left pinned rather than bumped in the same commit as a security fix:
	// forty-one minor versions of an RPC library is its own change, with its
	// own testing, and SECURITY.md's "the only thing that showed up was the
	// AI endpoint we had chosen" claim has to be re-verified against whatever
	// transports a newer version brings.
	github.com/mark3labs/mcp-go v0.17.0
	github.com/muesli/ansi v0.0.0-20230316100256-276c6243b2f6
	github.com/muesli/reflow v0.3.0
	github.com/muesli/termenv v0.16.0
	github.com/ncruces/go-sqlite3 v0.25.0
	github.com/openai/openai-go v0.1.0-beta.2
	github.com/pressly/goose/v3 v3.24.2
	github.com/sergi/go-diff v1.3.2-0.20230802210424-5b0b94c5c0d3
	github.com/spf13/cobra v1.9.1
	github.com/spf13/viper v1.20.0
	github.com/stretchr/testify v1.10.0
)

require (
	cloud.google.com/go v0.116.0 // indirect
	cloud.google.com/go/auth v0.13.0 // indirect
	cloud.google.com/go/compute/metadata v0.6.0 // indirect
	github.com/andybalholm/cascadia v1.3.2 // indirect
	github.com/atotto/clipboard v0.1.4 // indirect
	github.com/aws/aws-sdk-go-v2 v1.30.3 // indirect
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.6.3 // indirect
	github.com/aws/aws-sdk-go-v2/config v1.27.27 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.17.27 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.16.11 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.3.15 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.6.15 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.11.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.11.17 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.22.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.26.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.30.3 // indirect
	github.com/aws/smithy-go v1.20.3 // indirect
	github.com/aymanbagabas/go-osc52/v2 v2.0.1 // indirect
	github.com/aymerick/douceur v0.2.0 // indirect
	github.com/charmbracelet/colorprofile v0.2.3-0.20250311203215-f60798e515dc // indirect
	github.com/charmbracelet/x/cellbuf v0.0.13-0.20250311204145-2c3ea96c31dd
	github.com/charmbracelet/x/term v0.2.1 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/disintegration/imaging v1.6.2
	github.com/dlclark/regexp2 v1.11.4 // indirect
	github.com/erikgeiser/coninput v0.0.0-20211004153227-1c3628e74d0f // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.2.1 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/s2a-go v0.1.8 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.4 // indirect
	github.com/googleapis/gax-go/v2 v2.14.1 // indirect
	github.com/gorilla/css v1.0.1 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/lithammer/fuzzysearch v1.1.8
	github.com/lucasb-eyer/go-colorful v1.2.0
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-localereader v0.0.1 // indirect
	github.com/mattn/go-runewidth v0.0.16
	github.com/mfridman/interpolate v0.0.2 // indirect
	github.com/microcosm-cc/bluemonday v1.0.27 // indirect
	github.com/muesli/cancelreader v0.2.2 // indirect
	github.com/ncruces/julianday v1.0.0 // indirect
	github.com/pelletier/go-toml/v2 v2.2.3 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/rogpeppe/go-internal v1.14.1 // indirect
	github.com/sagikazarmark/locafero v0.7.0 // indirect
	github.com/sethvargo/go-retry v0.3.0 // indirect
	github.com/sourcegraph/conc v0.3.0 // indirect
	github.com/spf13/afero v1.12.0 // indirect
	github.com/spf13/cast v1.7.1 // indirect
	github.com/spf13/pflag v1.0.6 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	github.com/tetratelabs/wazero v1.9.0 // indirect
	github.com/tidwall/gjson v1.18.0 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/tidwall/sjson v1.2.5 // indirect
	github.com/xo/terminfo v0.0.0-20220910002029-abceb7e1c41e // indirect
	github.com/yosida95/uritemplate/v3 v3.0.2 // indirect
	github.com/yuin/goldmark v1.7.8 // indirect
	github.com/yuin/goldmark-emoji v1.0.5 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.54.0 // indirect
	go.opentelemetry.io/otel v1.35.0 // indirect
	go.opentelemetry.io/otel/metric v1.35.0 // indirect
	go.opentelemetry.io/otel/trace v1.35.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/crypto v0.37.0 // indirect
	golang.org/x/image v0.26.0 // indirect
	golang.org/x/net v0.39.0
	golang.org/x/sync v0.13.0 // indirect
	golang.org/x/sys v0.32.0 // indirect
	golang.org/x/term v0.31.0 // indirect
	golang.org/x/text v0.24.0 // indirect
	google.golang.org/genai v1.64.0
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250324211829-b45e905df463 // indirect
	google.golang.org/grpc v1.71.0 // indirect
	google.golang.org/protobuf v1.36.6 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
