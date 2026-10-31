# ffprobe (embedded toolbox)

FFprobe from FFmpeg 9.0, the same name on Windows and Linux. Reads what a video or audio file holds (length, codecs,
size, streams) and changes nothing. `-v error` hides the banner; `-of json` gives output a program can read.

    ffprobe -v error -show_format -show_streams -of json in.mp4                    # everything, as JSON
    ffprobe -v error -show_entries format=duration -of csv=p=0 in.mp4             # length in seconds
    ffprobe -v error -select_streams v:0 -show_entries stream=codec_name,width,height -of csv=p=0 in.mp4
