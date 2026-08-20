# Gorilla OpenCode v0.1.110 — a model list you can finally curate

Everything about this release is printed in full on this page, including the
pictures.

---

## The problem

Some providers offer hundreds of models. The list is long enough to be useless,
and there was no way to prune it. You could bookmark models one at a time, but
you could not **remove** one, could not **refresh** to see what a provider serves
today, and could not **start clean**.

A picker that cannot pick, purge or update is missing the parts that make it a
picker.

---

## What you can do now

| Key | What it does |
|---|---|
| `x` | Mark a model (and move down, so marking a run is one key per model) |
| `a` | Add every marked model to your list at once |
| `d` | Hide the marked models — or the highlighted one |
| `H` | Review what you hid. `d` there **restores** it |
| `space` | Bookmark, as before |

And two commands:

- **`/purge`** — empties the downloaded provider lists
- **`/update`** — asks each provider what it serves today

**Your own list and your hidden models survive both.**

---

## Why hiding had to be permanent

Nothing in this program had ever removed a model. The list is built from
providers compiled into the app and added to by every refresh. So a plain
deletion is undone twice over: the compiled-in models come back with the next
launch, and the fetched ones come back with the next refresh.

You would clear three hundred entries on Monday and find them back on Tuesday.

So hidden models are **recorded on disk and honoured by refresh**. That is what
makes `/purge` worth running.

**Nothing hides itself.** On the same day, five models we tried to benchmark came
back unreachable and we could not tell whether they were broken or whether our
own free allowance had run out. Automatic hiding would have deleted all five for
having a bad minute.

**And hiding is never a one-way door.** A `HIDDEN` column appears the moment
anything is hidden, and the key that hides is the key that restores.

---

## The picker uses the whole window

It was capped at thirty rows however large your screen. On a 1600x900 display
that threw away seven usable rows and pushed models behind a scroll — the exact
wall the list exists to remove. The cap is gone.

---

## An embarrassing fix

**The connection speed estimate added in v0.1.108 never ran. Not once.**

The code that measures your link was written, tested and shipped across two
releases, and nothing ever called it. The picker said *"nothing measured yet"*
permanently, and the feature meant to suggest a profile could never suggest
anything.

It works now. The repair comes with a test that sends a **real request through
the real code** and fails if no measurement is recorded — including a control
that reproduces the broken version, so the test cannot pass for the wrong
reason.

The earlier tests passed because they called the measuring function **directly**.
They proved the unit worked while never asking whether anything used it.

---

## Also published: models that run tools you did not ask for

A two-word greeting made one model search the web for **"Debian kernel
configuration for beginners"** — not one word of which came from the user. It was
assembled from the name of the folder the session was open in.

We checked whether our own instructions caused it. **They do not.** Removing whole
sections of them changed nothing, while other models given the identical
instructions behaved correctly:

| Model | Restraint | Readiness |
|---|---|---|
| `meta/llama-3.1-70b-instruct` | **80%** | **100%** |
| `meta/llama-3.3-70b-instruct` | **0%** | 100% |
| `meta/llama-3.1-8b-instruct` | **0%** | 100% |
| `nvidia/llama-3.3-nemotron-super-49b-v1.5` | 100% | **0%** |

Read the last row twice: perfect restraint, and it **never runs a tool when you
ask it to**. On a restraint-only table it would come first while being the least
useful model on the list.

Full write-up and the script to reproduce it:
[`docs/TOOL-DISCIPLINE.md`](https://github.com/gorillanobakaa-dot/Gorilla.Opencode/blob/v0.1.110/docs/TOOL-DISCIPLINE.md).

**We did not label these models in the picker.** Five others could not be reached,
and we could not tell a broken model from our own exhausted quota. A verdict on
that basis would be a guess wearing the authority of a test.

---

## What is NOT verified

- Multi-select, hiding and the hidden column are covered by unit tests and by
  reading — they have not been driven by hand in the interface.
- The benchmark is two runs per prompt against one provider on one day. Enough
  to separate 0% from 100%; not enough to argue 70% against 80%.
- One unrelated test fails when tests run in random order. It fails the same way
  on the previous release.

---

## Install

**Debian / Ubuntu**

```sh
sudo dpkg -i gorilla-opencode_0.1.110_amd64.deb
```

**Arch / CachyOS**

```sh
sudo pacman -U gorilla-opencode-0.1.110-1-x86_64.pkg.tar.zst
```

Verify what you downloaded against `checksums.txt` before installing.
