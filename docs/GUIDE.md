# A plain-English guide to the Gorilla OpenCode screen

This is for someone who has never used a tool like this. It explains
what you are looking at, what each menu means, and the one keyboard
trick that isn't obvious: how to switch to the Google (Gemini) models.

Gorilla OpenCode is an AI coding assistant that lives in a terminal —
the black window where you type commands. You talk to it in plain
English; it can read files, write files, and run commands to help you
build and fix software. It uses *your* API key, so you pay the AI
provider directly (or nothing, if you run a model on your own machine).

## The screen, top to bottom

**Top-left — the conversation.** Your messages and the assistant's
replies. Under each reply you'll see the model that answered and how
long it took, e.g. *"Nemotron 3 Super 120B (1ms)"*. That's just telling
you which "brain" replied.

**Top-right — the status panel:**
- **Gorilla OpenCode v0.1.x** and the GitHub link — which version you're
  running.
- **cwd: /home/you** — the folder ("current working directory") the
  assistant is working in. Everything it reads or writes is relative to
  here.
- **Session: …** — the name of this conversation. Each chat is a
  "session" and is saved.
- **LSP Configuration** — optional language-server help (autocomplete,
  error-checking). "LSPs are disabled" just means none is set up; it's
  not an error.
- **Modified Files** — files the assistant has changed in this session.
  "No modified files" means it hasn't touched anything yet.

**Bottom bar — the important numbers:**
- **ctrl+? help** — press Ctrl and ? together for the key list.
- **Context: 6.9K** — how much text (measured in "tokens", roughly
  ¾ of a word each) the assistant sends to the AI *every single message*
  — its instructions, its tools, and the conversation so far. Smaller is
  cheaper and faster. Early builds sent ~15K just to say "hi"; trimming
  that number is what the `/context` menu below is for.
- **Cost: $0.00** — what this session has cost. $0.00 here because the
  provider (NVIDIA NIM) is on a flat-rate key.
- **The highlighted name on the far right** (e.g. *Nemotron 3 Super
  120B*) — the model currently answering.

## The commands you type

Type these into the message box and press Enter:

- **/model** (or **/models**) — open the model picker (see below).
- **/context** — open the token-budget menu (see below).
- **/export** — save this whole conversation to a text file in your
  current folder.
- **/clear** — start a fresh conversation and forget the old one.

## The model picker — and how to reach the Google models

Open it with **/model**. You'll see a box titled **"Select Local Model"**
(or "Select Gemini Model", etc.) with a list of models and, at the
bottom-right, something like **`81/118  ← ↑ ↓`** or **`1/4  →`**.

Here's the part that isn't obvious:

- **↑ / ↓ (up/down arrows)** move through the models on the *current*
  provider. `81/118` means "model 81 of 118".
- **← / →  (left/right arrows)** switch between *providers*. Your models
  are grouped: all your NVIDIA NIM models on one page, all your Google
  Gemini models on another, local Ollama on another.

So **to use a Google model: open /model, then press the → (right arrow)
key** until the title changes to **"Select Gemini Model"**. You'll see
Gemini 2.0 Flash, Gemini Flash (latest), Gemini Pro (latest), and the
`1/4 →` at the bottom telling you you're on the Gemini page. Pick one
with ↑/↓ and press Enter. (`h` and `l` also work as left/right if you
prefer.)

The models are sorted best-for-coding first, and each shows a short
description — its size, context window, and how good it is at code — so
you can tell 118 of them apart.

## The context loadout (`/context`) — your token budget

Open it with **/context**. This is the "how much am I paying per
message, and for what" menu — total control, nothing hidden. It lists
everything sent to the AI every turn (its tools, its background
knowledge of your folder) with the token cost of each, and lets you
switch any of it off:

- **space** turns an item on or off. Turning it off saves those tokens.
- **⚠** marks items that cripple the assistant if removed (e.g. turning
  off "Bash" means it can't run any commands).
- **r** resets everything to the defaults if you strip too much.
- The header shows the running total — *"~7,778 tokens are sent to the
  model on EVERY turn, even to say yo"*.

The single biggest saving is usually **"Environment info"** — it feeds
the AI the git status of your whole folder every message, which in a
huge home directory can be ~9,000 tokens on its own. Turn it off when
you don't need it and the number drops immediately.

## Before and after

The screenshots in `gallery/` show the journey: early ones with the
context number up around 15,000 (a lot of baggage every message), and
later ones down around 7,000 after trimming the loadout and moving to a
leaner, modern system prompt. Same assistant, roughly half the overhead.
