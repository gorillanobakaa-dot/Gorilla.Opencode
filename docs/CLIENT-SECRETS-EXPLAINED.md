<p align="center"><img src="../internal/assets/icons/gorilla-opencode-256.png" width="96" alt="Gorilla OpenCode"></p>

<h1 align="center">Why did GitHub block our push over a "secret" that isn't secret?</h1>

<p align="center"><em>A true story from building this project — and everything it teaches about OAuth logins, "client secrets," and how to tell a real leak from a false alarm.</em></p>

---

## What actually happened

We added "Sign in with Google" to this app. We committed the code, pushed to
GitHub, and got slapped:

```
remote: - GITHUB PUSH PROTECTION
remote:     Push cannot contain secrets
remote:       —— Google OAuth Client ID ——
remote:          path: internal/auth/gemini_oauth.go:40
remote:       —— Google OAuth Client Secret ——
remote:          path: internal/auth/gemini_oauth.go:41
```

Two "secrets," push rejected. And yet — these particular values are **public
on purpose**. They're printed in Google's own open-source Gemini CLI for
anyone to read. So who's right, GitHub or us? *Both*, and understanding why
is a genuinely useful piece of knowledge that has nothing to do with this
app.

---

## For anyone (no jargon)

### First: what is "Sign in with Google," really?

You've clicked "Sign in with Google" on a hundred websites. Here's the
machinery under that button, in plain terms.

When an app wants to use your Google account, **three parties** are involved:
you, Google, and the app. They do a little dance:

1. The app sends you to a real Google page and says *"this is the FooBar app,
   it wants to see your email — is that OK?"*
2. You approve **on Google's own page** (the app never sees your password).
3. Google hands the app a **token** — a temporary key that works *only* for
   your account, *only* for what you approved.

For this to work, the app needs to **prove which app it is** when it talks to
Google. That's what the two flagged values are:

| Value | Plain meaning | Analogy |
|---|---|---|
| **Client ID** | the app's public name badge | a shop's name on the sign |
| **Client Secret** | a second string that also identifies the app | the shop's business-license number |

Neither of these is *your* anything. They identify **the app**, not the user.

### The twist: two kinds of "client secret"

Here's the part almost nobody explains. There are two flavours of app, and the
word "secret" means completely different things for each:

| App type | Example | Is the client secret really secret? |
|---|---|---|
| **Confidential** (runs on a server you control) | a website's backend | **Yes.** Lives on a locked server. If leaked, attackers can impersonate your app. Guard it. |
| **Public / installed** (runs on the user's machine) | a desktop app, a CLI, this app | **No — it can't be.** It ships *inside* a program anyone can download and unzip. |

The Gemini CLI is an **installed app**. Its "client secret" is baked into a
program millions of people download. Google's own documentation says the
secret for installed apps "is obviously not treated as confidential." It's a
secret in name only — like a "hidden" key under a doormat that the manual
tells everyone about. It doesn't unlock *your* account; on its own it unlocks
nothing.

### So what IS the real secret?

The thing that actually protects your account is the **token** Google gives
back *after you log in* — and that never leaves your computer.

| Thing | Is it sensitive? | Where does it live? | In our GitHub repo? |
|---|---|---|---|
| Your Google **password** | 🔴 Extremely | only you + Google know it; typed on Google's page | **Never touched** |
| Your **access / refresh token** | 🔴 Yes — the real key to your account | a `0600` file on *your* disk | **No** |
| The app's **client id + secret** | 🟢 Not really (installed app) | inside every copy of the program | Yes — and that's fine |

**This is the whole lesson:** "is it a secret?" is the wrong question. The
right question is *"what can someone actually do if they have it?"* With the
app's installed-app credentials: nothing, without also stealing a token off
your machine. With your token: everything — so that's the one kept local and
`0600`.

### Then why did GitHub freak out?

GitHub runs **secret scanning + push protection**: it reads every push and
pattern-matches for things that *look* like credentials — AWS keys, Stripe
keys, and yes, Google client secrets (which have a recognisable
`GOCSPX-…` shape).

It saw `GOCSPX-…` in our code and **stopped the push**. That's the correct,
safe default! But a scanner is a pattern-matcher — it **cannot tell** a
dangerous *web-app* secret from a harmless *installed-app* one. They look
identical. So it blocks both and asks a human to decide.

GitHub then gives you a link that says, in effect, *"I've looked; this one is
intentional and safe — allow it."* You click it, you see:

```
✔  Secret allowed
   You can now push this secret to the repository.
```

…and the push goes through. We used that path deliberately, because keeping
the value **visible with a comment explaining it's the public Gemini-CLI
value** is more honest than hiding it. (The alternative — chopping the string
up so the scanner can't recognise it — would just be *concealing a public
fact to fool a robot*. We didn't do that.)

### How to tell if a "leaked secret" actually matters

A rule of thumb you can carry anywhere:

1. **What is it?** A password / token / private-server key → 🔴 rotate it now.
2. **Where was it designed to live?** If it *ships inside software users
   download* (installed-app secret, a public API "key" meant for browsers),
   it was never confidential → probably 🟢, but confirm #3.
3. **What can it do alone?** If it needs a *second* thing you didn't leak (a
   user token, a password) to do any harm → low risk.
4. **When in doubt, rotate.** Rotating a credential is cheap. Assuming
   "it's probably fine" about a *real* secret is not.

---

## For developers

### The OAuth 2.0 installed-app flow (what our login does)

```
gorilla-opencode login
        │
        ├─ 1. start a localhost server on 127.0.0.1:<random>/oauth2callback
        ├─ 2. open the browser to accounts.google.com/o/oauth2/v2/auth
        │        ?client_id=<app id>&redirect_uri=<localhost>&scope=…
        │        &state=<random>&access_type=offline&prompt=consent
        ├─ 3. user approves on Google's page (app never sees the password)
        ├─ 4. Google redirects to the localhost server with ?code=…&state=…
        ├─ 5. verify state (CSRF guard), POST code → oauth2.googleapis.com/token
        │        with client_id + client_secret + code
        └─ 6. receive access_token (1h) + refresh_token (long-lived)
                 → store 0600, refresh access_token automatically
```

- The **client_id/secret** authenticate the *application* at step 5. For a
  confidential client you'd keep the secret server-side; for an installed app
  there is no server, so the secret is embedded and PKCE / the loopback
  redirect + `state` do the real anti-abuse work. Google issues these clients
  as "Desktop app" credentials precisely because the secret is non-secret.
- The **token** at step 6 is the sensitive artifact. It's user-scoped and
  short-lived (access) or revocable (refresh). It lives only on disk.

### Why the scanner can't distinguish them

A `GOCSPX-` prefixed string is the format for *all* Google OAuth client
secrets — the platform doesn't encode "confidential vs installed" into the
string. So GitHub's regex-plus-verification pipeline flags every one. The
`unblock-secret/<id>` URL records a human decision ("known, intentional") and
is the sanctioned way to push a legitimately-public value. In CI you can also
allowlist paths, but a one-off human ack is cleaner and auditable.

### What we did, concretely

- `internal/auth/gemini_oauth.go` holds the public Gemini-CLI client id +
  secret **with a comment** stating they're public installed-app values.
- The **user's** tokens go to `~/.config/gorilla-opencode/gemini-oauth.json`
  at `0600` and are **git-ignored by living outside the repo entirely**.
- We allowed the two scanner hits via GitHub's unblock URLs rather than
  obfuscating, so the code stays auditable.

### Verify every claim here yourself

- Read Google's own public client secret in the Gemini CLI source:
  search `github.com/google-gemini/gemini-cli` for `OAUTH_CLIENT_SECRET`.
- Confirm our app never writes tokens into the repo:
  `grep -rn "gemini-oauth.json" .` → it only ever references a path under
  `~/.config`.
- Confirm the only hosts touched are Google's own — see the live network
  audit in **[SECURITY.md](../SECURITY.md)**.

---

## The five things to walk away with

1. **"Sign in with Google" hands the app a token, never your password.** The
   token is the real key; it stays on your machine.
2. **A "client secret" identifies the *app*, not you.** For desktop/CLI apps
   it ships inside the program and is public by necessity.
3. **"Is it a secret?" is the wrong question.** Ask *"what can someone do with
   it alone?"* — that's your actual threat model.
4. **Secret scanners are pattern-matchers.** They can't tell safe from
   dangerous look-alikes, so they block both and ask a human. That's a
   feature, not a nuisance.
5. **When you bypass a scanner, do it in the open.** Allow the value with a
   comment explaining why, rather than hiding a public fact to fool the tool.

---

<p align="center"><em>This happened for real while building
<a href="../README.md">Gorilla OpenCode</a>. We wrote it down instead of
quietly clicking "allow," because explaining is the product.<br>
See also: <a href="GOOGLE-LOGIN.md">the Google login feature</a> ·
<a href="../SECURITY.md">the network-transparency audit</a>.</em></p>
