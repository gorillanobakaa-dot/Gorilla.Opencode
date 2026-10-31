# oxipng (embedded toolbox)

Make a PNG smaller WITHOUT changing a single pixel (lossless). It rewrites the file in place unless you give `--out`.

    oxipng -o 4 --strip safe in.png --out smaller.png   # keeps the original, writes a new file
    oxipng -o max --strip safe in.png --out smaller.png # slowest, smallest
    oxipng -o 4 --strip all -r pictures/                # a whole folder, IN PLACE; strips all metadata
    oxipng --version
