# v0.1.164, in plain language

## What this is

Quota meters that show every limit, and a clearer display of messages from other sessions.

## Should you download it?

Yes, if you use Gorilla OpenCode. Nothing you have set up is removed.

## Why this matters

- `/usage` shows the exact time left until each meter resets ("resets in 4h 59m" for the five-hour meter, "resets in 3d 15h" for a weekly one), the same way for every provider. Where the backend sends no reset time (a full, unused meter) none is shown, never a guess.
- `/usage` shows one quota meter for each limit the backend reports, including the Ultra-only one.
- A reading that is missing is shown as unknown, not as 0%; a model with no meter says why.
- Messages from other sessions (peers) are drawn as their own block with a header, apart from what you typed, and are never passed to the AI as your words.
- Every theme and the status bar now reach WCAG AA contrast, with tests.
- Peers work when the state folder path is long (shorter socket path).
- A debug line shows the reasoning effort that was sent.

## What was NOT checked

- The meters were tested against recorded readings, not a live account.
- The Windows program is unsigned.
