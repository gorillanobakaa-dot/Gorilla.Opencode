# ffmpeg (embedded toolbox)

Converts audio, video and pictures. Call it by name in the shell. `-y` overwrites the output, `-hide_banner` shortens
the output. To only read a video's length or streams use ffprobe.

    ffmpeg -y -i clip.mp4 -vn -c:a libmp3lame out.mp3     # extract the audio as mp3
    ffmpeg -y -i in.png out.webp                          # convert a picture
    ffmpeg -y -i clip.mp4 -vf scale=640:-2 small.mp4      # shrink a video to 640 wide
