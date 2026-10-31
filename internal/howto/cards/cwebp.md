# cwebp (embedded toolbox)

Encode a PNG, JPEG or TIFF as WebP (libwebp 1.6.0). Writes a NEW file; the source stays. See the `image` card for the whole picture workflow.

    cwebp -q 80 in.png -o out.webp                  # lossy, quality 0-100 (80 is a good start)
    cwebp -lossless in.png -o out.webp              # exact pixels, bigger
    cwebp -q 80 -resize 800 0 in.jpg -o small.webp  # width 800, height 0 = keep the proportions
    cwebp -q 80 -metadata none in.jpg -o out.webp   # drop EXIF/ICC/XMP (the default already drops them)
