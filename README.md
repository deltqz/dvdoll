# DVDoll - FFmpeg DVD remuxer
DVDoll is a guided CLI tool that walks you through the process of remuxing your DVDs to MKV files. No commands, no GUIs; just choose a file and the chapters to process. It also detects whether your audio is PCM or AC3/DTS/MP2. In the former case, it converts it to FLAC, otherwise it simply remuxes it.

<img width="961" height="520" alt="image" src="https://github.com/user-attachments/assets/ced8da45-9abe-4620-89fa-cde7f8059b8b" />

## How to use
- Upon opening DVDoll, simply paste the full path to your DVD ISO or folder when it asks for input.
- It will then list the titles in your DVD.
- Using that information, choose the appropriate title and chapters.
- Name your output file.

## Tips and must-knows
- You can also drag and drop a file or folder into your terminal at the input prompt.
- You can use a video player like mpv or MPC-HC to locate the chapters you need.
- If chapters are not reliable, you can use timecodes by typing `time` or `0` when DVDoll asks for the first chapter.
- You can use Aegisub or [this useful mpv script](https://github.com/Arieleg/mpv-copyTime/blob/master/copyTime.lua) to easily get the timecode of a frame by pressing Ctrl+C.
- `0`, `start` and leaving it blank are aliases for `00:00:00`.
- `last`, `end` and leaving it blank mean "until the end of the video".