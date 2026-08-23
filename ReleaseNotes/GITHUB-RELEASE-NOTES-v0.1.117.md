# Gorilla OpenCode v0.1.117: a window of nothing but a zero is not a window

Everything about this release is printed in full on this page, including the
pictures.

**One fix.** The usage panel was drawing a full green bar for an allowance that
does not exist.

---

## The bar that was measuring nothing

Sign in with a ChatGPT account, type `/usage`, and you got two bars: a monthly
limit reading 16% left, and underneath it a **"Secondary usage limit" at 100%,
three bananas, full green**.

There is no secondary limit on a free plan. That bar was measuring nothing.

[![The usage panel showing a real monthly limit at 16 percent remaining, and beneath it a phantom secondary usage limit drawn as a full green bar at 100 percent with three banana icons](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.117/docs/screenshots/gallery/v0117-before-phantom-secondary-limit.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.117/docs/screenshots/gallery/v0117-before-phantom-secondary-limit.png)

OpenAI's own client, on the same account, minutes apart, shows **only** the
monthly limit. That disagreement is what exposed it:

[![OpenAI's Codex status output on the same free account at the same time, listing a monthly limit and no secondary limit of any kind](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.117/docs/screenshots/gallery/v0117-codex-shows-only-monthly.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/v0.1.117/docs/screenshots/gallery/v0117-codex-shows-only-monthly.png)

---

## Where it came from

OpenAI's server does send a "secondary" figure on every reply. On a free account
it sends a bare **zero**, with no window length and no reset time attached. That
is the server's way of saying *not applicable here*.

The reading taken off the account above, exactly as stored:

```
primary  : used 84%,  window 30 days,  resets 15 September   <- a real limit
secondary: used 0%,   no window,       no reset time         <- nothing at all
```

Our code took that zero at face value. Zero used means a hundred left, so it drew
a hundred left.

**OpenAI's own program throws that reading away.** It counts a limit as real only
if at least one of the three things says something:

```rust
let has_data = used_percent != 0.0
    || window_minutes.is_some_and(|minutes| minutes != 0)
    || resets_at.is_some();
```

A bare zero fails all three. That is why the two screens disagreed.

---

## Why this is worse than showing nothing

A blank would have been honest. A confident full green bar is not.

This project has a rule written into the file that reads balances: **unknown and
plenty-left must never look alike.** A full bar for an allowance that does not
exist is the bad half of exactly that pair. Somebody could look at that screen
and believe they had a second pot of credit held in reserve.

---

## Fixed, using their guard rather than one of ours

A limit is now real if it has been used at all, **or** declares how long its
window is, **or** says when it resets. A bare zero is discarded.

Ported rather than invented, on the same principle as the window labels before
it: this backend is undocumented, and OpenAI's own client is the only authority
on what its headers mean.

**Both directions are tested**, because getting it wrong the other way is just as
bad. A limit reading zero used that **does** carry a window or a reset time is a
genuine untouched allowance, and it still shows.

| test | asserts |
|---|---|
| `TestAWindowOfNothingButZeroIsNotAWindow` | the bare-zero secondary is dropped, the real monthly one kept |
| `TestAnUntouchedButRealWindowSurvives` | zero used WITH a window, or WITH a reset, still renders |
| `TestOnlyAnEmptySecondaryMeansNoReading` | a reply carrying only the empty secondary parses to nothing, so "no meter" stays distinct from "meter reads full" |

Checked non-vacuously: removing the guard reproduces v0.1.116 and fails two of
the three, quoting the phantom window.

**If you had the old version**, the phantom bar disappears on your next request,
when the cached reading is rewritten from fresh headers. Nothing to do.


**Fixed, on the installed build.** Same account, same panel, no second bar:

[![The usage panel on the installed v0.1.117 build showing the monthly limit at 16 percent remaining standing alone, with no secondary usage limit bar beneath it at all](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/dacfe79270f3cfaa5b46d6f707818f5789540053/docs/screenshots/gallery/v0117-after-monthly-limit-alone.png)](https://raw.githubusercontent.com/gorillanobakaa-dot/Gorilla.Opencode/dacfe79270f3cfaa5b46d6f707818f5789540053/docs/screenshots/gallery/v0117-after-monthly-limit-alone.png)

The version in frame reads v0.1.117, so the picture carries its own proof of
which build produced it. That one image is pinned to a commit rather than to the
tag, because it was taken after the tag was cut: a commit hash is immutable, so
the picture on this page can never change under it.

---

## A note on the two numbers disagreeing

Our panel said 16% and Codex said 18% at the same moment. **That one is not a
fault.** Both read the same monthly pot, but each updates from *its own* replies,
and about 2% had been spent through this program since Codex last looked. Ours
was the fresher figure.

---

## What was checked, and what was not

Verified: full test suite green; the new guard checked non-vacuously by
reinstating the bug; the packaged binary's SHA-256 equal to the built binary and
to the installed copy.

**Not verified, said plainly:**

- **Any paid ChatGPT plan.** A paid account reports a genuine second window,
  which by the guard's own rule is kept, but nobody here has one to check
  against.
- **That the 16% vs 18% gap is purely freshness.** It is the obvious explanation
  and both figures move on their own client's traffic, but it was not proven by
  forcing a request and re-reading both.

---

## Install

**Debian / Ubuntu:**

```sh
sudo apt install ./gorilla-opencode_0.1.117_amd64.deb
```

`apt` rather than `dpkg -i`, because the package depends on `lynx`, `python3`
and `ripgrep`, and `dpkg` resolves nothing.

**Arch / CachyOS**, pre-built, no Go toolchain needed:

```sh
sudo pacman -U gorilla-opencode-0.1.117-1-x86_64.pkg.tar.zst
```

Or build from source: `git clone`, `cd packaging`, `makepkg -si`. The PKGBUILD
carries a real checksum, not `SKIP`, so `makepkg` verifies what it fetched.

**Verify your download first**, in the same directory as the files:

```sh
sha256sum -c SHA256SUMS-v0.1.117.txt
```
