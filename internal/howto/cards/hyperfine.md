# hyperfine (embedded toolbox)

Measures how long commands take: runs each several times, warms up, reports mean, spread and the ratio.
On Windows it starts each command through cmd.exe unless `-N` (no shell, then the command must be a program).

    hyperfine --warmup 3 'pfind TODO src' 'git grep TODO -- src'
    hyperfine -N --runs 20 'go version'
    hyperfine --export-markdown bench.md 'cmd /c build.bat'
Heavy benchmarks load every core: say how long it will take before starting one.
