// Package assets carries build-time embedded resources — the application
// icons — so a single downloaded binary can install itself completely,
// with no companion files to fetch or lose.
//
// GORILLA OVERRIDE: this package did not exist upstream. It exists so
// that users who are not comfortable hunting down icon files and desktop
// entries by hand get the exact same result as users who are.
package assets

import "embed"

// Icons holds the application icon in the sizes shipped to
// hicolor icon themes, plus the scalable SVG master.
//
//go:embed icons/*
var Icons embed.FS

// IconSizes lists the raster sizes available in Icons, matching
// files named icons/gorilla-opencode-<size>.png.
var IconSizes = []int{128, 256, 512, 1024}
