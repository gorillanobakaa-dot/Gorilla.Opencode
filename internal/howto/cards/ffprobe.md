# ffprobe (embedded toolbox)

Reads a media file's facts without changing it. Call it by name in the shell.

    ffprobe -v error -show_entries format=duration -of default=nw=1:nk=1 clip.mp4   # length in seconds
    ffprobe -hide_banner clip.mp4                                                    # streams, codecs, size
    ffprobe -v error -show_streams -of json clip.mp4                                 # everything, as JSON
