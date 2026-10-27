# ninja (embedded toolbox)

The small fast build tool CMake, Meson and GN generate files for (build.ninja). It only runs; it does not configure:
run CMake or Meson first.

    ninja -C build                 # build everything in the build folder
    ninja -C build -n              # dry run: what would be rebuilt
    ninja -C build -t targets all  # list the targets
`ninja -C build -t clean` deletes the build outputs. Use `-j N` to limit the parallel jobs on a hot machine.
