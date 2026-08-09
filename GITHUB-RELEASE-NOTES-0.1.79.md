## lynx is now required, not suggested

One fix, straight after v0.1.78.

That release declared `Recommends: lynx` — the small text browser that makes web
search work with no setup. It sounded polite and it was wrong.

| Install method | Honours `Recommends:`? |
|---|---|
| `apt install ./file.deb` | yes |
| **gdebi** (right-click → Open With) | **no** — its source never mentions the field |
| `dpkg -i` | no — resolves nothing at all |

So the two friendliest routes, the ones used by people who don't live in a
terminal, would have silently skipped the package that makes the headline feature
work. Web search would have looked broken to exactly the audience this is built
for. A promise that only holds when you install the expert way is not a promise.

`Recommends: lynx` → **`Depends: lynx`**. It is 641 KB and now arrives on every
install path.

```sh
sudo apt install ./Compiled.Builds/gorilla-opencode_0.1.79_amd64.deb
# or right-click the .deb → Open With → GDebi
```

`dpkg -i` will now refuse until lynx is present — correct behaviour: loud, rather
than quietly installing something that cannot do what it advertises.

**Not verified:** the gdebi path was not exercised. The conclusion comes from
reading its source, and lynx was already installed on the test machine, so a
clean-machine gdebi install remains untested.
