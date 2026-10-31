# avifenc (embedded toolbox)

Encode a PNG or JPEG as AVIF (libavif 1.4.2, aom inside). Smaller than WebP at the same look, but slow to encode. Writes a NEW file.

    avifenc -q 60 in.png out.avif                   # quality 0-100 (60 is a good start)
    avifenc -q 60 -s 6 in.png out.avif              # speed 0 (slowest, smallest) to 10 (fastest)
    avifenc --lossless in.png out.avif              # exact pixels
    avifenc -q 60 -j all in.png out.avif            # use every core
