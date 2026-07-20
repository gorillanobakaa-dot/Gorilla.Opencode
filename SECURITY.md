# Security & Network Transparency

**Short version:** Gorilla OpenCode talks to the AI provider *you* configure
and to nothing else. No telemetry, no analytics, no phone-home, no backdoor.
This page does not ask you to believe that — it shows you how to **prove it
yourself** in about two minutes, and shows the result when we did.

Two layers of evidence: reading the source (what the code *can* do) and
watching the running program with forensic tools (what it *actually* does).

---

## For anyone (no jargon)

A "coding agent" is a program that reads your files and sends them to an AI
model to get help. The fair question is: *does it also quietly send my data
somewhere else — usage stats, my code, a tracking beacon?* Lots of "free"
tools pay for themselves exactly that way.

The honest way to answer isn't a promise in a README. It's to put the
program under a microscope while it runs and watch every network connection
it makes. Your computer already has those microscopes (`ss`, `tshark`,
`strace`). Below is how to point them at this app. If it were spying on you,
a connection to some server you didn't configure would show up. When we ran
it, **the only thing that showed up was the AI endpoint we had chosen.**

---

## Layer 1 — What the source code can reach

Every outbound connection in the code falls into three buckets, all of which
*you* switch on:

1. **AI provider endpoints** — only the ones you give a key or endpoint to:
   NVIDIA NIM (`integrate.api.nvidia.com`), Groq, Cerebras, OpenRouter, xAI,
   Azure, and the SDK-default hosts for Anthropic (`api.anthropic.com`),
   OpenAI (`api.openai.com`) and Google Gemini
   (`generativelanguage.googleapis.com`). GitHub Copilot
   (`api.github.com` + `api.githubcopilot.com`) only if you pick Copilot.
2. **Your own `LOCAL_ENDPOINT`** (Ollama / a local NIM) for listing models.
3. **One optional third party:** `sourcegraph.com` — a public code-search
   tool the model can invoke. It sends the *search query*, not your code, and
   only if the model uses that tool.

Reproduce the source scan:

```sh
# Every hardcoded connection host in our own code (excludes doc-comment links):
grep -rnoE 'https?://[a-zA-Z0-9._-]+' internal/ cmd/ main.go --include=*.go \
  | grep -viE 'microsoft|github.io|language-server|json-schema|w3.org|wikipedia|visualstudio|draculatheme'

# Any analytics/telemetry SDK? (expect: none)
grep -rniE 'posthog|sentry|segment|mixpanel|amplitude|datadog|analytics|telemetry' \
  internal/ cmd/ --include=*.go | grep -viE 'no.?telemetry|disable|// '

# Any hardcoded IP address? (expect: none but loopback)
grep -rnoE '\b([0-9]{1,3}\.){3}[0-9]{1,3}\b' internal/ cmd/ --include=*.go
```

Notes from our own run:
- **No analytics/telemetry SDK is present.** The `go.opentelemetry.io/*`
  lines in `go.mod` are `// indirect` — pulled in transitively by cloud SDKs
  and **never initialized or exported**. There is no tracer/meter provider
  and no exporter endpoint anywhere in our code.
- **No hardcoded IPs, no phone-home on startup** — no update check, no
  version beacon. The only init-time network call queries *your* local
  endpoint's model list.

---

## Layer 2 — What the running program actually does

Source reading tells you what *could* happen. This tells you what *does*.
Watch the live process with the same tools a forensic analyst would.

> **The one discipline that matters:** capture tools like `tshark -i any` see
> your **whole machine** — your browser, your chat apps, everything. That is
> not this app. You must **attribute connections to the process by its PID**
> (the kernel knows which process owns each socket). Skip this step and you
> will blame the coding agent for your browser's traffic.

### Step 1 — find the process

```sh
PID=$(pgrep -f '/gorilla-opencode$' | head -1); echo "PID=$PID"
```

### Step 2 — watch ONLY that process's connections while you use it

```sh
# Poll the process's own established sockets once a second for 40s.
# Give the agent a real task in the app while this runs.
for i in $(seq 1 40); do
  sudo ss -tnHp state established 2>/dev/null | grep "pid=$PID,"
  sleep 1
done | grep -oE '[0-9a-f.:]+:443' | sort -u
```

Every IP that prints is a place the app connected to. There should be no
surprises — only your provider.

### Step 3 — name each IP and confirm who owns it

```sh
for ip in <the IPs from step 2>; do
  echo ">> $ip"; getent hosts "$ip"; whois "$ip" | grep -iE 'OrgName|netname'
done
```

### Optional — see the domain on the wire (TLS SNI) and every new connection

```sh
# Domain names in the TLS handshake (whole machine — cross-reference the IPs from step 2):
sudo tshark -i any -f 'tcp port 443' -Y 'tls.handshake.type==1' \
  -T fields -e ip.dst -e tls.handshake.extensions_server_name

# Every new connect() syscall the process makes, attributed to it:
sudo strace -f -e trace=connect -yy -p "$PID"
```

(`strace` may show no *new* connect during streaming because the HTTPS
socket is kept alive and reused — that is why the socket-polling in Step 2 is
the reliable method; it sees the live connection regardless.)

---

## Result of our own audit

Config: NVIDIA NIM. An agent was left running a real task for the capture
window. Filtered to the sockets the process actually owned:

| Remote IP | Owner (whois) | Identity |
|---|---|---|
| `99.83.136.103:443` | Amazon (AWS Global Accelerator) | TLS **SNI = `integrate.api.nvidia.com`** |
| `75.2.113.119:443` | Amazon (AWS Global Accelerator) | 2nd A-record of `integrate.api.nvidia.com` |

`dig integrate.api.nvidia.com` returns those two exact IPs. NVIDIA fronts its
API through AWS Global Accelerator, which is why `whois` says Amazon — that is
NVIDIA's own CDN layer, not a detour through anyone else.

**Across the entire window the process's distinct-remote-IP list was those
two and nothing else.** No telemetry host, no analytics, no unexpected
domain. Domains like `cloud.google.com` and `api.github.com` that appeared in
the *whole-machine* SNI capture were other applications (a browser, a desktop
chat app) and disappeared the instant we attributed traffic to the PID.

### Honest scope of this proof

- It is a **snapshot of one session on one config**. It shows the app talks
  to the provider you selected and nothing extra. Enable another provider and
  it will talk to **that** provider — by design. Re-run the audit any time;
  the commands above are all you need.
- We audited **our** first-party code (`internal/`, `cmd/`, `main.go`) in
  full. We did **not** line-by-line audit every transitive dependency's
  source; the vendored provider SDKs connect to their own documented
  endpoints and none are analytics libraries, but that is a statement about
  their purpose, not a line-by-line proof of every dep.
- The optional `sourcegraph.com` code-search tool only connects if the model
  invokes it; it did not in our window.

---

## Reporting a vulnerability

Found something that contradicts the above, or any other security issue?
Please open an issue at
<https://github.com/gorillanobakaa-dot/Gorilla.Opencode/issues>, or if it is
sensitive, note it there without exploit details and ask for a private
channel. This is a small volunteer project; there is no bug bounty, but
credible reports are taken seriously and disclosed transparently, which is
the whole point of this page.
