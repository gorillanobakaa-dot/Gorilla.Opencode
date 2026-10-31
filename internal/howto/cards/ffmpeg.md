# ffmpeg (embedded toolbox)

FFmpeg 9.0 (static, GPL build), the same name on Windows and Linux. Converts, cuts and joins video and audio. It
never changes the input; it writes the output file named LAST, and asks before overwriting it (`-n` refuses, `-y`
overwrites). `-c copy` cuts or re-wraps without re-encoding (fast, no quality loss). Look at a file first with ffprobe.

    ffmpeg -n -i in.mov -c:v libx264 -crf 23 -c:a aac out.mp4       # convert to a widely playable MP4
    ffmpeg -n -ss 00:01:00 -to 00:02:30 -i in.mp4 -c copy clip.mp4   # cut 1:00-2:30, no re-encode
    ffmpeg -n -i in.mp4 -vn -c:a libmp3lame -q:a 2 audio.mp3         # extract the sound as MP3
    ffmpeg -n -i in.mp4 -vf scale=1280:-2 small.mp4                   # resize to 1280 wide, keep the ratio
