# ffmpeg (embedded toolbox)

Converts audio, video and pictures. Call it by name in the shell. `-y` overwrites the output, `-hide_banner` shortens
the output. To only read a video's length or streams use ffprobe.

    ffmpeg -y -i clip.mp4 -vn -c:a libmp3lame out.mp3     # extract the audio as mp3
    ffmpeg -y -i in.png out.webp                          # convert a picture
    ffmpeg -y -i clip.mp4 -vf scale=640:-2 small.mp4      # shrink a video to 640 wide

Speech to text: `whisper-cli` (whisper.cpp) is NOT in the toolbox (its official builds are several files plus a model of
75 MB or more: third_party/PINS.md). If present: `whisper-cli -m ggml-tiny.en.bin -f audio.wav`; make the 16 kHz wav
with `ffmpeg -y -i in.mp4 -ar 16000 -ac 1 audio.wav`. A person fetches the model from the whisper.cpp project page.
