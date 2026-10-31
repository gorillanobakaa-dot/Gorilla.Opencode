# Pictures: convert, resize, compress, OCR, metadata (how-to)

Never overwrite the original: always write a new file name, then look at the result (size, and the picture itself).
Toolbox programs (cwebp, avifenc, oxipng) are called by name. `magick` (ImageMagick 7) and `tesseract` are NOT in the
toolbox: check with `magick -version` / `tesseract --version`; if missing, a person installs them (Windows:
`scoop install imagemagick tesseract`; Debian/Ubuntu: `sudo apt install imagemagick tesseract-ocr`).
JPEG has no toolbox encoder (mozjpeg ships no official binary for both systems): use `magick ... -quality 82`.

    # convert
    cwebp -q 80 in.png -o out.webp                       # PNG/JPEG -> WebP
    avifenc -q 60 in.png out.avif                        # PNG/JPEG -> AVIF
    magick in.webp out.png                               # anything else, either direction

    # resize (keeps the proportions; the > only ever shrinks)
    magick in.jpg -resize 1280x1280\> out.jpg
    cwebp -q 80 -resize 800 0 in.png -o small.webp       # resize while encoding

    # compress at EQUAL quality: shrink, then measure that it still looks the same
    oxipng -o 4 --strip safe in.png --out smaller.png    # PNG, lossless: identical pixels
    magick compare -metric SSIM in.png out.webp null:    # ImageMagick 7.1.2: 0 = identical, so SMALLER is closer;
                                                         # read the number in brackets (0-1); under 0.02 is hard to see
    magick compare -metric PSNR in.png out.webp null:    # higher is closer; above 40 dB is very close
    # Try quality 90, 80, 70... and keep the smallest file whose bracketed SSIM stays under your bar (say 0.02).
    # Check the direction on two identical files first: the same command prints 0 for them.

    # OCR (text in a picture); language eng, add -l deu for German, etc.
    tesseract page.png stdout -l eng                     # prints the text
    tesseract page.png out -l eng                        # writes out.txt

    # metadata (EXIF: camera, date, GPS)
    magick identify -format '%[EXIF:*]' photo.jpg        # read the EXIF tags
    magick identify -verbose photo.jpg                   # everything, long
    magick photo.jpg -strip clean.jpg                    # copy without EXIF/GPS/ICC (GPS = where it was taken)
    oxipng --strip all in.png --out clean.png            # the same for a PNG, lossless

A phone photo's EXIF often holds the GPS position: strip it before a picture leaves the machine.
