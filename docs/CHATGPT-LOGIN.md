<p align="center"><img src="../internal/assets/icons/gorilla-opencode-256.png" width="96" alt="Gorilla OpenCode"></p>

<h1 align="center">Sign in with ChatGPT — OpenAI models, no API key</h1>

<p align="center"><em>A free ChatGPT account is enough. No developer account, no payment method, no credit card.</em></p>

---

## For anyone (why this exists)

OpenAI gives you two doors and only advertises one of them.

The advertised door is an **API key**. To get a key you need an OpenAI
*developer* account, and to finish that you need a payment method. If you do not
have a card — or you are fifteen, or the card in your house is not yours to use —
that door is shut. Not expensive: shut.

The other door is the **ChatGPT account you already have**. That account can
reach OpenAI's coding models directly, and a **free** ChatGPT account works. You
sign in through your browser, the way you sign in to any website, and the models
appear.

There is **no per-use charge on this route at all.** That is why the model picker
shows no prices next to these two entries. Your ChatGPT plan is what pays, and if
your plan is the free one you pay nothing. Use too much and you are *paused* for a
while — never billed for going over.

That is the whole point: the cost of entry drops from a credit card to an email
address.

### How to use it

Start the program — click the icon in your applications menu, or run
`gorilla-opencode` — and the sign-in list appears. **ChatGPT is the second row**,
tagged `free`:

```
  Antigravity free tier - Claude + GPT-OSS + Gemini (Gmail sign-in)  free
> ChatGPT sign-in - GPT-5.5 (works on the FREE plan, no API key)  free
  Google - Code Assist free tier (Gemini only, Gmail sign-in, no key)  free
```

Move to it with the arrow keys and press **Enter**. Your browser opens OpenAI's
normal sign-in page — sign in as you always do and approve the request. The
browser says you are signed in; go back to the terminal, which reports:

```
Signed in as you@example.com (plan: free).
```

From a terminal instead, if you prefer:

```sh
gorilla-opencode login --chatgpt
```

### Which models you get

| Model | What it is for |
|---|---|
| **GPT-5.5** | Your actual work. 272K context, tools, images. Selected for you. |
| **GPT-5.4 Mini** | Background jobs you never see — naming a conversation, summarising an old one. |

The program picks the smaller model for background work on purpose. A ChatGPT
plan has **one** shared allowance, so spending the strong model on titling a chat
brings your pause forward for no benefit you can see.

Switch models any time with `/model`.

### Why GPT-5.6 is missing

The service also advertises **GPT-5.6 Terra** and **GPT-5.6 Luna**, and this
program does not offer them. That is deliberate.

Both report a setting called `code_mode_only`: they expect the program to hand
them tools in a shape it does not speak yet. If we listed them, you would sign in
successfully, pick one, and watch it fail the first time it tried to read a file.
An entry that looks like it works and does not is worse than no entry — so we
left them out, and the sign-in screen says so on the row itself, so their absence
does not look like a bug on your machine.

Implementing that tool shape is outstanding work.

---

## Honest limits

**This is unofficial.** OpenAI documents signing in to *their* tool with a
ChatGPT plan. They document nothing about other programs doing it. This speaks
the same protocol their client speaks, and it identifies itself truthfully as
`gorilla_opencode` — it does not pretend to be OpenAI's own tool. But it is not a
published, supported interface, so an OpenAI-side change can break it without
warning.

**GPT-5.4 Mini has a death date.** OpenAI stops serving GPT-5.4 to ChatGPT
sign-ins on **31 August 2026**. Nothing here breaks; the model is withdrawn. Use
GPT-5.5.

**There is no usage meter.** The Antigravity sign-in has a screen showing how much
allowance is left (`/usage`). We have not found an equivalent for this one, and
showing you an invented number would be worse than showing none.

**The model list does not refresh itself.** It is a snapshot of what the service
reported on 17 August 2026. When OpenAI changes the lineup, it takes a new
release to notice.

---

## What is stored, and where

Signing in saves a token from OpenAI at:

```
~/.config/gorilla-opencode/chatgpt-oauth.json
```

Readable only by you (mode `0600`). It is a **separate file** from the two Google
sign-ins, so none of the three can overwrite another. You can hold all three at
once, and that is worth doing: they are different accounts with different
allowances, so when one pauses you, the others do not.

The sign-in uses **PKCE**, the standard protection that stops another program on
your machine from stealing the code your browser hands back. The callback lands on
`localhost:1455` — a fixed port, because OpenAI registered that exact address and
rejects any other one before you ever see a prompt. If something else on your
machine is already using port 1455, sign-in fails immediately and says so.

Every request asks the server **not** to keep a copy of your conversation.

To sign out, delete the file:

```sh
rm ~/.config/gorilla-opencode/chatgpt-oauth.json
```

To sign in again as a different account:

```sh
gorilla-opencode login --chatgpt --relogin
```

---

## For developers

The transport is `internal/llm/provider/chatgpt.go`; the login is
`internal/auth/chatgpt_oauth.go`; the catalogue is
`internal/llm/models/chatgpt.go`. Each file's header explains the decisions.

The backend is `chatgpt.com/backend-api/codex` and it speaks the **Responses
API**, not Chat Completions. It is *not* `OpenAIClient` behind a different base
URL the way GROQ, xAI and DeepSeek are — the token does not authenticate against
`api.openai.com` at all, and the wire format differs in message shape, history
vocabulary, tool encoding and event stream.

Two facts about this backend cost real time to find and are documented nowhere
citable:

- It **rejects** `max_output_tokens` with `400 Unsupported parameter`. The public
  Responses API accepts it.
- `client_version` is a **required query parameter** on `/responses`, not a
  header. Omitting it returns a 400 naming `('query','client_version')`.

There are live probes, skipped unless you opt in, because the wire format cannot
be verified any other way:

```sh
CHATGPT_LIVE=1 go test ./internal/llm/provider/ -run TestChatGPTLive -v -count=1
```

They need a real signed-in credential and they spend a little of your plan's
allowance. Run them after **any** change to the request shape — every field in it
was an inference, and two inferences were wrong until these ran.

---

*See also: [Sign in with Google](GOOGLE-LOGIN.md) · [Why two documentation tracks?](../PHILOSOPHY.md)*
